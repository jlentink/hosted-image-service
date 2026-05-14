<?php
/**
 * Media library integration.
 *
 * Hooks into WordPress to use the image-service for generating thumbnails
 * and provides regeneration functionality.
 */

if ( ! defined( 'ABSPATH' ) ) {
    exit;
}

class WPIR_Media_Handler {

    public function __construct() {
        // Register our image editor as the preferred one.
        add_filter( 'wp_image_editors', array( $this, 'register_image_editor' ) );

        // Admin hooks for regeneration.
        if ( is_admin() ) {
            add_action( 'admin_menu', array( $this, 'add_regenerate_page' ) );
            add_filter( 'bulk_actions-upload', array( $this, 'register_bulk_action' ) );
            add_filter( 'handle_bulk_actions-upload', array( $this, 'handle_bulk_action' ), 10, 3 );
            add_action( 'admin_notices', array( $this, 'bulk_action_notice' ) );
            add_action( 'wp_ajax_wpir_regenerate_single', array( $this, 'ajax_regenerate_single' ) );
            add_action( 'wp_ajax_wpir_regenerate_batch', array( $this, 'ajax_regenerate_batch' ) );
        }
    }

    /**
     * Prepend our image editor to the list so WordPress uses it first.
     */
    public function register_image_editor( $editors ) {
        array_unshift( $editors, 'WPIR_Image_Editor' );
        return $editors;
    }

    /**
     * Add the regeneration admin page under Media.
     */
    public function add_regenerate_page() {
        add_submenu_page(
            'upload.php',
            __( 'Regenerate Thumbnails', 'wp-image-resizer' ),
            __( 'Regenerate Thumbnails', 'wp-image-resizer' ),
            'manage_options',
            'wpir-regenerate',
            array( $this, 'render_regenerate_page' )
        );
    }

    /**
     * Register bulk action in media library.
     */
    public function register_bulk_action( $actions ) {
        $actions['wpir_regenerate'] = __( 'Regenerate Thumbnails (Image Service)', 'wp-image-resizer' );
        return $actions;
    }

    /**
     * Handle bulk regenerate action.
     */
    public function handle_bulk_action( $redirect_url, $action, $post_ids ) {
        if ( 'wpir_regenerate' !== $action ) {
            return $redirect_url;
        }

        $regenerated = 0;
        foreach ( $post_ids as $post_id ) {
            $result = $this->regenerate_attachment( $post_id );
            if ( ! is_wp_error( $result ) ) {
                $regenerated++;
            }
        }

        return add_query_arg( 'wpir_regenerated', $regenerated, $redirect_url );
    }

    /**
     * Show admin notice after bulk action.
     */
    public function bulk_action_notice() {
        if ( ! empty( $_REQUEST['wpir_regenerated'] ) ) {
            $count = intval( $_REQUEST['wpir_regenerated'] );
            printf(
                '<div class="notice notice-success is-dismissible"><p>%s</p></div>',
                sprintf(
                    _n(
                        '%d image regenerated via Image Service.',
                        '%d images regenerated via Image Service.',
                        $count,
                        'wp-image-resizer'
                    ),
                    $count
                )
            );
        }
    }

    /**
     * AJAX handler: regenerate a single attachment.
     */
    public function ajax_regenerate_single() {
        check_ajax_referer( 'wpir_regenerate', 'nonce' );

        if ( ! current_user_can( 'manage_options' ) ) {
            wp_send_json_error( __( 'Permission denied.', 'wp-image-resizer' ) );
        }

        $attachment_id = isset( $_POST['attachment_id'] ) ? intval( $_POST['attachment_id'] ) : 0;
        if ( ! $attachment_id ) {
            wp_send_json_error( __( 'Invalid attachment ID.', 'wp-image-resizer' ) );
        }

        $result = $this->regenerate_attachment( $attachment_id );

        if ( is_wp_error( $result ) ) {
            wp_send_json_error( $result->get_error_message() );
        }

        wp_send_json_success( array(
            'attachment_id' => $attachment_id,
            'sizes'         => count( $result ),
        ) );
    }

    /**
     * AJAX handler: regenerate a batch of attachments.
     */
    public function ajax_regenerate_batch() {
        check_ajax_referer( 'wpir_regenerate', 'nonce' );

        if ( ! current_user_can( 'manage_options' ) ) {
            wp_send_json_error( __( 'Permission denied.', 'wp-image-resizer' ) );
        }

        $offset = isset( $_POST['offset'] ) ? intval( $_POST['offset'] ) : 0;
        $limit  = isset( $_POST['limit'] ) ? intval( $_POST['limit'] ) : 10;

        $attachments = get_posts( array(
            'post_type'      => 'attachment',
            'post_mime_type' => array( 'image/jpeg', 'image/png', 'image/webp', 'image/gif', 'image/avif' ),
            'posts_per_page' => $limit,
            'offset'         => $offset,
            'post_status'    => 'inherit',
            'fields'         => 'ids',
        ) );

        $total = wp_count_posts( 'attachment' );
        $total_images = isset( $total->inherit ) ? $total->inherit : 0;

        $results = array();
        foreach ( $attachments as $attachment_id ) {
            $result = $this->regenerate_attachment( $attachment_id );
            $results[] = array(
                'id'      => $attachment_id,
                'success' => ! is_wp_error( $result ),
                'message' => is_wp_error( $result ) ? $result->get_error_message() : 'OK',
            );
        }

        wp_send_json_success( array(
            'results'     => $results,
            'processed'   => $offset + count( $attachments ),
            'total'       => $total_images,
            'has_more'    => count( $attachments ) === $limit,
        ) );
    }

    /**
     * Regenerate all thumbnails for a given attachment.
     *
     * @param int $attachment_id
     * @return array|WP_Error Array of generated sizes or error.
     */
    public function regenerate_attachment( $attachment_id ) {
        $file = get_attached_file( $attachment_id );
        if ( ! $file || ! file_exists( $file ) ) {
            return new WP_Error( 'wpir_missing_file', __( 'Original file not found.', 'wp-image-resizer' ) );
        }

        // Remove old resized files.
        $old_metadata = wp_get_attachment_metadata( $attachment_id );
        if ( ! empty( $old_metadata['sizes'] ) ) {
            $upload_dir = dirname( $file );
            foreach ( $old_metadata['sizes'] as $size_data ) {
                $size_file = $upload_dir . '/' . $size_data['file'];
                if ( file_exists( $size_file ) ) {
                    wp_delete_file( $size_file );
                }
            }
        }

        // Regenerate metadata (triggers our image editor).
        $metadata = wp_generate_attachment_metadata( $attachment_id, $file );

        if ( is_wp_error( $metadata ) ) {
            return $metadata;
        }

        wp_update_attachment_metadata( $attachment_id, $metadata );

        return isset( $metadata['sizes'] ) ? $metadata['sizes'] : array();
    }

    /**
     * Render the regenerate thumbnails admin page.
     */
    public function render_regenerate_page() {
        if ( ! current_user_can( 'manage_options' ) ) {
            return;
        }

        $nonce = wp_create_nonce( 'wpir_regenerate' );
        ?>
        <div class="wrap">
            <h1><?php esc_html_e( 'Regenerate Thumbnails', 'wp-image-resizer' ); ?></h1>
            <p><?php esc_html_e( 'Regenerate all image thumbnails using the external image service. This will process all images in your media library.', 'wp-image-resizer' ); ?></p>

            <?php if ( ! wpir_is_configured() ) : ?>
                <div class="notice notice-error">
                    <p><?php esc_html_e( 'Image Resizer is not configured. Please set the service URL and JWT secret in Settings > Image Resizer.', 'wp-image-resizer' ); ?></p>
                </div>
            <?php else : ?>
                <div id="wpir-regenerate-progress" style="display:none;">
                    <div class="wpir-progress-bar" style="background:#f0f0f0; border:1px solid #ccc; border-radius:4px; height:30px; width:100%; max-width:600px; margin:20px 0;">
                        <div id="wpir-progress-fill" style="background:#0073aa; height:100%; width:0%; border-radius:4px; transition:width 0.3s;"></div>
                    </div>
                    <p id="wpir-progress-text"><?php esc_html_e( 'Starting...', 'wp-image-resizer' ); ?></p>
                    <div id="wpir-progress-log" style="max-height:300px; overflow-y:auto; background:#f9f9f9; padding:10px; border:1px solid #ddd; font-family:monospace; font-size:12px;"></div>
                </div>

                <p>
                    <button type="button" id="wpir-start-regenerate" class="button button-primary">
                        <?php esc_html_e( 'Regenerate All Thumbnails', 'wp-image-resizer' ); ?>
                    </button>
                </p>

                <script>
                (function() {
                    var btn = document.getElementById('wpir-start-regenerate');
                    var progress = document.getElementById('wpir-regenerate-progress');
                    var fill = document.getElementById('wpir-progress-fill');
                    var text = document.getElementById('wpir-progress-text');
                    var log = document.getElementById('wpir-progress-log');
                    var nonce = '<?php echo esc_js( $nonce ); ?>';
                    var running = false;

                    btn.addEventListener('click', function() {
                        if (running) return;
                        running = true;
                        btn.disabled = true;
                        btn.textContent = '<?php echo esc_js( __( 'Processing...', 'wp-image-resizer' ) ); ?>';
                        progress.style.display = 'block';
                        processBatch(0);
                    });

                    function processBatch(offset) {
                        var data = new FormData();
                        data.append('action', 'wpir_regenerate_batch');
                        data.append('nonce', nonce);
                        data.append('offset', offset);
                        data.append('limit', 5);

                        fetch(ajaxurl, { method: 'POST', body: data })
                            .then(function(r) { return r.json(); })
                            .then(function(resp) {
                                if (!resp.success) {
                                    text.textContent = 'Error: ' + (resp.data || 'Unknown error');
                                    running = false;
                                    btn.disabled = false;
                                    return;
                                }

                                var d = resp.data;
                                var pct = d.total > 0 ? Math.round((d.processed / d.total) * 100) : 100;
                                fill.style.width = pct + '%';
                                text.textContent = d.processed + ' / ' + d.total + ' processed (' + pct + '%)';

                                d.results.forEach(function(r) {
                                    var line = document.createElement('div');
                                    line.textContent = '#' + r.id + ': ' + r.message;
                                    line.style.color = r.success ? '#006600' : '#cc0000';
                                    log.appendChild(line);
                                    log.scrollTop = log.scrollHeight;
                                });

                                if (d.has_more) {
                                    processBatch(d.processed);
                                } else {
                                    text.textContent = 'Done! ' + d.processed + ' images processed.';
                                    btn.textContent = '<?php echo esc_js( __( 'Regenerate All Thumbnails', 'wp-image-resizer' ) ); ?>';
                                    btn.disabled = false;
                                    running = false;
                                }
                            })
                            .catch(function(err) {
                                text.textContent = 'Network error: ' + err.message;
                                running = false;
                                btn.disabled = false;
                            });
                    }
                })();
                </script>
            <?php endif; ?>
        </div>
        <?php
    }
}
