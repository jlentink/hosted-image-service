<?php
/**
 * Custom WP_Image_Editor that delegates processing to the image-service.
 *
 * This replaces WordPress's built-in GD/Imagick editors for resize/crop
 * operations while falling back to the default editor for unsupported operations.
 */

if ( ! defined( 'ABSPATH' ) ) {
    exit;
}

// Ensure the abstract base class is loaded — WordPress loads it lazily
// (only when image editing is first needed) so it may not be present yet
// during plugin activation or early plugin file inclusion.
if ( ! class_exists( 'WP_Image_Editor' ) ) {
    require_once ABSPATH . WPINC . '/class-wp-image-editor.php';
}

class WPIR_Image_Editor extends WP_Image_Editor {

    /** @var WPIR_API_Client */
    private $client;

    /** @var string Current file path. */
    protected $file;

    /** @var array Image dimensions. */
    protected $size;

    /** @var string Original mime type. */
    protected $mime_type;

    /**
     * Check if this editor supports the given mime type.
     */
    public static function test( $args = array() ) {
        // We support the same types the service supports.
        return wpir_is_configured();
    }

    /**
     * Check if this editor supports the given mime type.
     */
    public static function supports_mime_type( $mime_type ) {
        $supported = array(
            'image/jpeg',
            'image/png',
            'image/webp',
            'image/avif',
            'image/gif', // Accept GIF input, convert to other format.
        );
        return in_array( $mime_type, $supported, true );
    }

    /**
     * Load the image file.
     */
    public function load() {
        if ( ! is_file( $this->file ) ) {
            return new WP_Error( 'error_loading_image', __( 'File not found.', 'wp-image-resizer' ) );
        }

        $size = wp_getimagesize( $this->file );
        if ( ! $size ) {
            return new WP_Error( 'invalid_image', __( 'Could not read image size.', 'wp-image-resizer' ) );
        }

        $this->size      = array(
            'width'  => $size[0],
            'height' => $size[1],
        );
        $this->mime_type = $size['mime'];
        $this->client    = new WPIR_API_Client();

        return true;
    }

    /**
     * Resize the image.
     */
    public function resize( $max_w, $max_h, $crop = false ) {
        if ( ! $max_w && ! $max_h ) {
            return new WP_Error( 'image_resize_error', __( 'Width and height must be specified.', 'wp-image-resizer' ) );
        }

        // Calculate dimensions like WordPress does.
        $dims = image_resize_dimensions(
            $this->size['width'],
            $this->size['height'],
            $max_w,
            $max_h,
            $crop
        );

        if ( ! $dims ) {
            return new WP_Error( 'error_getting_dimensions', __( 'Could not calculate resized image dimensions.', 'wp-image-resizer' ) );
        }

        list( $dst_x, $dst_y, $src_x, $src_y, $dst_w, $dst_h, $src_w, $src_h ) = $dims;

        // Determine crop mode.
        // First check if we have a stored crop mode/focal point from the crop editor.
        $crop_mode = $this->get_stored_crop_mode();
        if ( null === $crop_mode ) {
            $crop_mode = 'center';
            if ( is_array( $crop ) && count( $crop ) === 2 ) {
                // WordPress passes crop as array like ['center', 'top'] or ['left', 'center'].
                $crop_mode = $this->wp_crop_to_focal_point( $crop );
            } elseif ( $crop ) {
                $crop_mode = 'center';
            }
        }

        $format  = $this->get_output_format();
        $quality = $this->get_quality_for_format( $format );

        $result = $this->client->resize( $this->file, $dst_w, $dst_h, $crop_mode, $format, $quality );

        if ( is_wp_error( $result ) ) {
            return $result;
        }

        // Store the result for save().
        $this->processed_data = $result['data'];
        $this->size = array(
            'width'  => $dst_w,
            'height' => $dst_h,
        );

        return true;
    }

    /**
     * Multi-resize: generate multiple sizes at once.
     */
    public function multi_resize( $sizes ) {
        $metadata = array();

        foreach ( $sizes as $size => $size_data ) {
            $width  = isset( $size_data['width'] ) ? (int) $size_data['width'] : 0;
            $height = isset( $size_data['height'] ) ? (int) $size_data['height'] : 0;
            $crop   = isset( $size_data['crop'] ) ? $size_data['crop'] : false;

            if ( ! $width && ! $height ) {
                continue;
            }

            // Reset state for each size.
            $this->processed_data = null;

            $result = $this->resize( $width, $height, $crop );
            if ( is_wp_error( $result ) ) {
                continue;
            }

            $resized = $this->_save_to_file( null, $size );
            if ( is_wp_error( $resized ) || ! $resized ) {
                continue;
            }

            // Restore original size for next iteration.
            $orig_size = wp_getimagesize( $this->file );
            if ( $orig_size ) {
                $this->size = array(
                    'width'  => $orig_size[0],
                    'height' => $orig_size[1],
                );
            }

            $metadata[ $size ] = array(
                'file'      => wp_basename( $resized['path'] ),
                'width'     => $resized['width'],
                'height'    => $resized['height'],
                'mime-type' => $resized['mime-type'],
                'filesize'  => isset( $resized['filesize'] ) ? $resized['filesize'] : filesize( $resized['path'] ),
            );
        }

        return $metadata;
    }

    /**
     * Crop the image (not resize+crop, just crop).
     */
    public function crop( $src_x, $src_y, $src_w, $src_h, $dst_w = null, $dst_h = null, $src_abs = false ) {
        // For pure crop operations, calculate focal point from crop area.
        $orig_w = $this->size['width'];
        $orig_h = $this->size['height'];

        if ( null === $dst_w ) {
            $dst_w = $src_w;
        }
        if ( null === $dst_h ) {
            $dst_h = $src_h;
        }

        // Calculate focal point from center of crop area.
        $focal_x = ( $src_x + $src_w / 2 ) / $orig_w;
        $focal_y = ( $src_y + $src_h / 2 ) / $orig_h;
        $focal_x = max( 0.0, min( 1.0, $focal_x ) );
        $focal_y = max( 0.0, min( 1.0, $focal_y ) );

        $crop_mode = sprintf( '%.4f,%.4f', $focal_x, $focal_y );
        $format    = $this->get_output_format();
        $quality   = $this->get_quality_for_format( $format );

        $result = $this->client->resize( $this->file, $dst_w, $dst_h, $crop_mode, $format, $quality );
        if ( is_wp_error( $result ) ) {
            return $result;
        }

        $this->processed_data = $result['data'];
        $this->size = array(
            'width'  => $dst_w,
            'height' => $dst_h,
        );

        return true;
    }

    /**
     * Rotate (not supported — fall back silently).
     */
    public function rotate( $angle ) {
        return true;
    }

    /**
     * Flip (not supported — fall back silently).
     */
    public function flip( $horz, $vert ) {
        return true;
    }

    /**
     * Stream the image (for previews).
     */
    public function stream( $mime_type = null ) {
        if ( ! empty( $this->processed_data ) ) {
            $ct = $mime_type ?: $this->get_output_mime_type();
            header( 'Content-Type: ' . $ct );
            echo $this->processed_data;
            return true;
        }
        return new WP_Error( 'image_stream_error', __( 'No processed image data.', 'wp-image-resizer' ) );
    }

    /**
     * Save the processed image to a file.
     */
    public function save( $destfilename = null, $mime_type = null ) {
        return $this->_save_to_file( $destfilename );
    }

    /**
     * Internal save method.
     */
    private function _save_to_file( $destfilename = null, $size_name = null ) {
        if ( empty( $this->processed_data ) ) {
            return new WP_Error( 'image_save_error', __( 'No processed image data to save.', 'wp-image-resizer' ) );
        }

        $format = $this->get_output_format();
        $ext    = $this->format_to_extension( $format );
        $mime   = $this->format_to_mime( $format );

        if ( null === $destfilename ) {
            $destfilename = $this->generate_filename( $ext );
        }

        // Ensure directory exists.
        wp_mkdir_p( dirname( $destfilename ) );

        $written = file_put_contents( $destfilename, $this->processed_data );
        if ( false === $written ) {
            return new WP_Error( 'image_save_error', __( 'Failed to write image file.', 'wp-image-resizer' ) );
        }

        // Set correct file permissions.
        $stat  = stat( dirname( $destfilename ) );
        $perms = $stat['mode'] & 0000666;
        chmod( $destfilename, $perms );

        return array(
            'path'      => $destfilename,
            'file'      => wp_basename( $destfilename ),
            'width'     => $this->size['width'],
            'height'    => $this->size['height'],
            'mime-type' => $mime,
            'filesize'  => $written,
        );
    }

    /**
     * Generate output filename based on dimensions.
     */
    protected function generate_filename( $extension = null, $dest_path = null, $suffix = '' ) {
        if ( null === $extension ) {
            $extension = $this->format_to_extension( $this->get_output_format() );
        }

        if ( ! $suffix ) {
            $suffix = $this->size['width'] . 'x' . $this->size['height'];
        }

        return parent::generate_filename( $suffix, $dest_path, $extension );
    }

    /**
     * Convert WordPress crop array to focal point string.
     */
    private function wp_crop_to_focal_point( $crop ) {
        $x_map = array( 'left' => 0.0, 'center' => 0.5, 'right' => 1.0 );
        $y_map = array( 'top' => 0.0, 'center' => 0.5, 'bottom' => 1.0 );

        $x = isset( $x_map[ $crop[0] ] ) ? $x_map[ $crop[0] ] : 0.5;
        $y = isset( $y_map[ $crop[1] ] ) ? $y_map[ $crop[1] ] : 0.5;

        if ( $x == 0.5 && $y == 0.5 ) {
            return 'center';
        }

        return sprintf( '%.1f,%.1f', $x, $y );
    }

    /**
     * Get the output format from plugin settings.
     */
    protected function get_output_format( $filename = null, $mime_type = null ) {
        return get_option( 'wpir_default_format', 'jpeg' );
    }

    /**
     * Get quality for a given format.
     */
    private function get_quality_for_format( $format ) {
        return (int) get_option( 'wpir_quality_' . $format, 0 );
    }

    /**
     * Get output mime type.
     */
    private function get_output_mime_type() {
        return $this->format_to_mime( $this->get_output_format() );
    }

    private function format_to_extension( $format ) {
        $map = array(
            'jpeg' => 'jpg',
            'png'  => 'png',
            'webp' => 'webp',
            'avif' => 'avif',
        );
        return isset( $map[ $format ] ) ? $map[ $format ] : 'jpg';
    }

    private function format_to_mime( $format ) {
        $map = array(
            'jpeg' => 'image/jpeg',
            'png'  => 'image/png',
            'webp' => 'image/webp',
            'avif' => 'image/avif',
        );
        return isset( $map[ $format ] ) ? $map[ $format ] : 'image/jpeg';
    }

    /**
     * Get stored crop mode from attachment meta (set via crop editor UI).
     *
     * @return string|null Crop mode string for the API, or null if not set.
     */
    private function get_stored_crop_mode() {
        // Try to find the attachment ID from the file path.
        $attachment_id = $this->get_attachment_id_from_file();
        if ( ! $attachment_id ) {
            return null;
        }

        return WPIR_Crop_Editor::get_crop_mode_for_service( $attachment_id );
    }

    /**
     * Look up the attachment ID from the current file path.
     */
    private function get_attachment_id_from_file() {
        if ( empty( $this->file ) ) {
            return 0;
        }

        $upload_dir = wp_get_upload_dir();
        $base_dir   = $upload_dir['basedir'];

        // Strip upload base to get relative path.
        if ( strpos( $this->file, $base_dir ) === 0 ) {
            $relative = ltrim( str_replace( $base_dir, '', $this->file ), '/' );
            global $wpdb;
            $attachment_id = $wpdb->get_var(
                $wpdb->prepare(
                    "SELECT post_id FROM {$wpdb->postmeta} WHERE meta_key = '_wp_attached_file' AND meta_value = %s LIMIT 1",
                    $relative
                )
            );
            return $attachment_id ? (int) $attachment_id : 0;
        }

        return 0;
    }

    /** @var string|null Processed image binary data. */
    private $processed_data;
}
