<?php
/**
 * Crop editor integration.
 *
 * Adds a focal point picker and crop mode selector to the media library.
 * Stores focal point and crop mode as attachment post meta.
 */

if ( ! defined( 'ABSPATH' ) ) {
    exit;
}

class WPIR_Crop_Editor {

    /** Meta keys. */
    const META_CROP_MODE = '_wpir_crop_mode';
    const META_FOCAL_X   = '_wpir_focal_x';
    const META_FOCAL_Y   = '_wpir_focal_y';

    public function __construct() {
        add_action( 'admin_enqueue_scripts', array( $this, 'enqueue_assets' ) );
        add_filter( 'wp_prepare_attachment_for_js', array( $this, 'add_meta_to_js' ), 10, 3 );
        add_action( 'wp_ajax_wpir_save_focal_point', array( $this, 'ajax_save_focal_point' ) );
    }

    /**
     * Enqueue crop editor JS and CSS on media pages.
     */
    public function enqueue_assets( $hook ) {
        if ( ! in_array( $hook, array( 'upload.php', 'post.php', 'post-new.php' ), true ) ) {
            return;
        }

        wp_enqueue_style(
            'wpir-crop-editor',
            WPIR_PLUGIN_URL . 'assets/css/crop-editor.css',
            array(),
            WPIR_VERSION
        );

        wp_enqueue_script(
            'wpir-crop-editor',
            WPIR_PLUGIN_URL . 'assets/js/crop-editor.js',
            array( 'jquery', 'media-views' ),
            WPIR_VERSION,
            true
        );

        wp_localize_script( 'wpir-crop-editor', 'wpirL10n', array(
            'nonce'            => wp_create_nonce( 'wpir_focal_point' ),
            'cropModeLabel'    => __( 'Crop Mode', 'wp-image-resizer' ),
            'center'           => __( 'Center', 'wp-image-resizer' ),
            'smart'            => __( 'Smart', 'wp-image-resizer' ),
            'focalPoint'       => __( 'Focal Point', 'wp-image-resizer' ),
            'clickToSetFocal'  => __( 'Click on the image to set the focal point:', 'wp-image-resizer' ),
            'save'             => __( 'Save', 'wp-image-resizer' ),
            'saveAndRegenerate' => __( 'Save & Regenerate', 'wp-image-resizer' ),
            'saved'            => __( 'Saved!', 'wp-image-resizer' ),
        ) );
    }

    /**
     * Add our meta to the attachment JS object so the crop editor can read it.
     */
    public function add_meta_to_js( $response, $attachment, $meta ) {
        $response['wpir_meta'] = array(
            'crop_mode' => get_post_meta( $attachment->ID, self::META_CROP_MODE, true ) ?: 'center',
            'focal_x'   => get_post_meta( $attachment->ID, self::META_FOCAL_X, true ) ?: '0.5',
            'focal_y'   => get_post_meta( $attachment->ID, self::META_FOCAL_Y, true ) ?: '0.5',
        );
        return $response;
    }

    /**
     * AJAX handler: save focal point and crop mode.
     */
    public function ajax_save_focal_point() {
        check_ajax_referer( 'wpir_focal_point', 'nonce' );

        if ( ! current_user_can( 'upload_files' ) ) {
            wp_send_json_error( __( 'Permission denied.', 'wp-image-resizer' ) );
        }

        $attachment_id = isset( $_POST['attachment_id'] ) ? intval( $_POST['attachment_id'] ) : 0;
        if ( ! $attachment_id || 'attachment' !== get_post_type( $attachment_id ) ) {
            wp_send_json_error( __( 'Invalid attachment.', 'wp-image-resizer' ) );
        }

        $crop_mode = isset( $_POST['crop_mode'] ) ? sanitize_text_field( $_POST['crop_mode'] ) : 'center';
        $focal_x   = isset( $_POST['focal_x'] ) ? floatval( $_POST['focal_x'] ) : 0.5;
        $focal_y   = isset( $_POST['focal_y'] ) ? floatval( $_POST['focal_y'] ) : 0.5;
        $regenerate = isset( $_POST['regenerate'] ) && $_POST['regenerate'] === '1';

        // Validate.
        $allowed_modes = array( 'center', 'smart', 'focal' );
        if ( ! in_array( $crop_mode, $allowed_modes, true ) ) {
            $crop_mode = 'center';
        }
        $focal_x = max( 0.0, min( 1.0, $focal_x ) );
        $focal_y = max( 0.0, min( 1.0, $focal_y ) );

        // Save meta.
        update_post_meta( $attachment_id, self::META_CROP_MODE, $crop_mode );
        update_post_meta( $attachment_id, self::META_FOCAL_X, number_format( $focal_x, 4, '.', '' ) );
        update_post_meta( $attachment_id, self::META_FOCAL_Y, number_format( $focal_y, 4, '.', '' ) );

        // Optionally regenerate.
        if ( $regenerate && wpir_is_configured() ) {
            $handler = new WPIR_Media_Handler();
            $result  = $handler->regenerate_attachment( $attachment_id );
            if ( is_wp_error( $result ) ) {
                wp_send_json_error( $result->get_error_message() );
            }
        }

        wp_send_json_success( array(
            'attachment_id' => $attachment_id,
            'crop_mode'     => $crop_mode,
            'focal_x'       => $focal_x,
            'focal_y'       => $focal_y,
        ) );
    }

    /**
     * Get the crop mode string for the image service.
     *
     * Used by the image editor when generating thumbnails.
     *
     * @param int $attachment_id
     * @return string Crop mode string for the API: 'center', 'smart', or 'x,y'.
     */
    public static function get_crop_mode_for_service( $attachment_id ) {
        $mode = get_post_meta( $attachment_id, self::META_CROP_MODE, true );

        switch ( $mode ) {
            case 'smart':
                return 'smart';

            case 'focal':
                $x = floatval( get_post_meta( $attachment_id, self::META_FOCAL_X, true ) ?: 0.5 );
                $y = floatval( get_post_meta( $attachment_id, self::META_FOCAL_Y, true ) ?: 0.5 );
                return sprintf( '%.4f,%.4f', $x, $y );

            case 'center':
            default:
                return 'center';
        }
    }
}
