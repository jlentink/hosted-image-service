<?php

/**
 * Crop editor integration.
 *
 * Adds a focal point picker and crop mode selector to the media library.
 * Stores focal point and crop mode as attachment post meta.
 * Per-size crop overrides are stored as JSON in _wpir_size_focal_{size_name} meta.
 */

if ( ! defined( 'ABSPATH' ) ) {
    exit;
}

class WPIR_Crop_Editor {

    /** Meta keys. */
    const META_CROP_MODE         = '_wpir_crop_mode';
    const META_FOCAL_X           = '_wpir_focal_x';
    const META_FOCAL_Y           = '_wpir_focal_y';
    const META_SIZE_FOCAL_PREFIX = '_wpir_size_focal_';

    public function __construct() {
        add_action( 'admin_enqueue_scripts', array( $this, 'enqueue_assets' ) );
        add_filter( 'wp_prepare_attachment_for_js', array( $this, 'add_meta_to_js' ), 10, 3 );
        add_action( 'wp_ajax_wpir_save_focal_point', array( $this, 'ajax_save_focal_point' ) );
        add_action( 'wp_ajax_wpir_save_size_focal',  array( $this, 'ajax_save_size_focal' ) );
    }

    /**
     * Enqueue crop editor JS and CSS on media pages.
     */
    public function enqueue_assets( $hook ) {
        if ( ! in_array( $hook, array( 'upload.php', 'post.php', 'post-new.php', 'media_page_wpir-crops' ), true ) ) {
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
            'nonce'             => wp_create_nonce( 'wpir_focal_point' ),
            'cropModeLabel'     => __( 'Crop Mode', 'wp-image-resizer' ),
            'center'            => __( 'Center', 'wp-image-resizer' ),
            'smart'             => __( 'Smart', 'wp-image-resizer' ),
            'focalPoint'        => __( 'Focal Point', 'wp-image-resizer' ),
            'clickToSetFocal'   => __( 'Click on the image to set the focal point:', 'wp-image-resizer' ),
            'save'              => __( 'Save', 'wp-image-resizer' ),
            'saveAndRegenerate' => __( 'Save & Regenerate', 'wp-image-resizer' ),
            'saved'             => __( 'Saved!', 'wp-image-resizer' ),
            'cropPreviewsTitle' => __( 'Crop Previews', 'wp-image-resizer' ),
            'smartPreviewNote'  => __( 'Smart crop — preview not available', 'wp-image-resizer' ),
            'adjustCrop'        => __( 'Adjust Crop', 'wp-image-resizer' ),
            'saveCrop'          => __( 'Save Crop', 'wp-image-resizer' ),
            'clearCrop'         => __( 'Clear Override', 'wp-image-resizer' ),
            'cancelCrop'        => __( 'Cancel', 'wp-image-resizer' ),
            'cropSaved'         => __( 'Crop saved!', 'wp-image-resizer' ),
            'regenerating'      => __( 'Regenerating…', 'wp-image-resizer' ),
            'dragHint'          => __( 'Drag to reposition:', 'wp-image-resizer' ),
            'customBadge'       => __( 'Custom', 'wp-image-resizer' ),
            'imageSizes'        => self::get_image_sizes_for_js(),
            'manageCrops'       => __( 'Manage Crops →', 'wp-image-resizer' ),
            'cropPageBase'      => admin_url( 'upload.php?page=wpir-crops' ),
        ) );
    }

    /**
     * Return registered image sizes that use cropping, formatted for JS.
     *
     * Only sizes with crop !== false are included because those are the ones
     * where focal point placement actually matters.
     *
     * @return array[] Each element: { name, label, width, height }
     */
    public static function get_image_sizes_for_js() {
        $registered = function_exists( 'wp_get_registered_image_subsizes' )
            ? wp_get_registered_image_subsizes()
            : array();

        $result = array();
        foreach ( $registered as $name => $data ) {
            // crop === false means scale-to-fit — focal point has no effect.
            if ( empty( $data['crop'] ) ) {
                continue;
            }

            $width  = isset( $data['width'] )  ? (int) $data['width']  : 0;
            $height = isset( $data['height'] ) ? (int) $data['height'] : 0;

            // Skip degenerate sizes.
            if ( $width <= 0 || $height <= 0 ) {
                continue;
            }

            $result[] = array(
                'name'   => $name,
                'label'  => ucwords( str_replace( array( '-', '_' ), ' ', $name ) ),
                'width'  => $width,
                'height' => $height,
            );
        }

        return $result;
    }

    /**
     * Add our meta to the attachment JS object so the crop editor can read it.
     */
    public function add_meta_to_js( $response, $attachment, $meta ) {
        // Per-size focal overrides: { size_name: { x: 0.3, y: 0.45 } }
        $size_focals = array();
        foreach ( self::get_image_sizes_for_js() as $size ) {
            $key = self::META_SIZE_FOCAL_PREFIX . $size['name'];
            $raw = get_post_meta( $attachment->ID, $key, true );
            if ( $raw ) {
                $data = json_decode( $raw, true );
                if ( $data && isset( $data['x'], $data['y'] ) ) {
                    $size_focals[ $size['name'] ] = $data;
                }
            }
        }

        $response['wpir_meta'] = array(
            'crop_mode'   => get_post_meta( $attachment->ID, self::META_CROP_MODE, true ) ?: 'center',
            'focal_x'     => get_post_meta( $attachment->ID, self::META_FOCAL_X,   true ) ?: '0.5',
            'focal_y'     => get_post_meta( $attachment->ID, self::META_FOCAL_Y,   true ) ?: '0.5',
            // Cast to object so PHP encodes {} (not []) when the array is empty.
            'size_focals' => (object) $size_focals,
        );
        return $response;
    }

    /**
     * AJAX handler: save global focal point and crop mode.
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
     * AJAX handler: save or clear a per-size focal point override.
     *
     * POST params:
     *   attachment_id  int
     *   size_name      string  (sanitize_key)
     *   focal_x        float   (0.0–1.0)
     *   focal_y        float   (0.0–1.0)
     *   clear          '0'|'1' — if '1', delete the override
     */
    public function ajax_save_size_focal() {
        check_ajax_referer( 'wpir_focal_point', 'nonce' );

        if ( ! current_user_can( 'upload_files' ) ) {
            wp_send_json_error( __( 'Permission denied.', 'wp-image-resizer' ) );
        }

        $attachment_id = isset( $_POST['attachment_id'] ) ? intval( $_POST['attachment_id'] ) : 0;
        if ( ! $attachment_id || 'attachment' !== get_post_type( $attachment_id ) ) {
            wp_send_json_error( __( 'Invalid attachment.', 'wp-image-resizer' ) );
        }

        $size_name = isset( $_POST['size_name'] ) ? sanitize_key( $_POST['size_name'] ) : '';
        if ( ! $size_name ) {
            wp_send_json_error( __( 'Invalid size name.', 'wp-image-resizer' ) );
        }

        $meta_key = self::META_SIZE_FOCAL_PREFIX . $size_name;
        $clear    = isset( $_POST['clear'] ) && '1' === $_POST['clear'];

        if ( $clear ) {
            delete_post_meta( $attachment_id, $meta_key );
            wp_send_json_success( array(
                'attachment_id' => $attachment_id,
                'size_name'     => $size_name,
                'cleared'       => true,
            ) );
        }

        $focal_x = isset( $_POST['focal_x'] ) ? floatval( $_POST['focal_x'] ) : 0.5;
        $focal_y = isset( $_POST['focal_y'] ) ? floatval( $_POST['focal_y'] ) : 0.5;
        $focal_x = max( 0.0, min( 1.0, $focal_x ) );
        $focal_y = max( 0.0, min( 1.0, $focal_y ) );

        update_post_meta(
            $attachment_id,
            $meta_key,
            wp_json_encode( array( 'x' => $focal_x, 'y' => $focal_y ) )
        );

        $regenerated = false;
        if ( isset( $_POST['regenerate'] ) && '1' === $_POST['regenerate'] && wpir_is_configured() ) {
            $handler = new WPIR_Media_Handler();
            $result  = $handler->regenerate_attachment( $attachment_id );
            if ( is_wp_error( $result ) ) {
                wp_send_json_error( $result->get_error_message() );
            }
            $regenerated = true;
        }

        wp_send_json_success( array(
            'attachment_id' => $attachment_id,
            'size_name'     => $size_name,
            'focal_x'       => $focal_x,
            'focal_y'       => $focal_y,
            'regenerated'   => $regenerated,
        ) );
    }

    /**
     * Get the crop mode string for the image service, checking per-size override first.
     *
     * @param int         $attachment_id
     * @param string|null $size_name  Registered size name (e.g. 'thumbnail'), or null.
     * @return string Crop mode string: 'center', 'smart', or 'x,y'.
     */
    public static function get_crop_mode_for_service_and_size( $attachment_id, $size_name ) {
        if ( $size_name ) {
            $raw = get_post_meta( $attachment_id, self::META_SIZE_FOCAL_PREFIX . $size_name, true );
            if ( $raw ) {
                $data = json_decode( $raw, true );
                if ( $data && isset( $data['x'], $data['y'] ) ) {
                    return sprintf( '%.4f,%.4f', (float) $data['x'], (float) $data['y'] );
                }
            }
        }
        // Fall back to the global crop mode / focal point.
        return self::get_crop_mode_for_service( $attachment_id );
    }

    /**
     * Get the crop mode string for the image service (global, no per-size override).
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
