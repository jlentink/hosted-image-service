<?php
/**
 * Dedicated per-image crop management page.
 *
 * Adds a "Manage Crops" row action to the media library list view and
 * renders a standalone admin page where editors can:
 *   - See every registered image size with its actual on-disk thumbnail.
 *   - Adjust the global crop mode (center / smart / focal point).
 *   - Fine-tune per-size manual crop rectangles via the shared JS tool.
 *   - Regenerate all thumbnails for the attachment in one click.
 */

if ( ! defined( 'ABSPATH' ) ) {
    exit;
}

class WPIR_Crop_Page {

    public function __construct() {
        add_action( 'admin_menu',        array( $this, 'register_page' ) );
        add_filter( 'media_row_actions', array( $this, 'add_row_action' ), 10, 2 );
    }

    /**
     * Register the hidden submenu page under Media.
     * An empty display-title string hides it from the sidebar.
     */
    public function register_page() {
        add_submenu_page(
            'upload.php',
            __( 'Manage Crops', 'wp-image-resizer' ),
            '',
            'upload_files',
            'wpir-crops',
            array( $this, 'render_page' )
        );
    }

    /**
     * Inject a "Manage Crops" link into every image row in the media list view.
     */
    public function add_row_action( $actions, $post ) {
        if ( 'attachment' !== $post->post_type ) {
            return $actions;
        }
        if ( strpos( $post->post_mime_type, 'image/' ) !== 0 ) {
            return $actions;
        }
        $url = admin_url( 'upload.php?page=wpir-crops&attachment_id=' . $post->ID );
        $actions['wpir_crops'] = sprintf(
            '<a href="%s">%s</a>',
            esc_url( $url ),
            esc_html__( 'Manage Crops', 'wp-image-resizer' )
        );
        return $actions;
    }

    /**
     * Render the full crop management page for one attachment.
     */
    public function render_page() {
        if ( ! current_user_can( 'upload_files' ) ) {
            wp_die( esc_html__( 'You do not have permission to manage image crops.', 'wp-image-resizer' ) );
        }

        $attachment_id = isset( $_GET['attachment_id'] ) ? intval( $_GET['attachment_id'] ) : 0;

        if ( ! $attachment_id || 'attachment' !== get_post_type( $attachment_id ) ) {
            wp_safe_redirect( admin_url( 'upload.php' ) );
            exit;
        }

        $post = get_post( $attachment_id );
        if ( ! $post || strpos( $post->post_mime_type, 'image/' ) !== 0 ) {
            wp_safe_redirect( admin_url( 'upload.php' ) );
            exit;
        }

        // --- Metadata ---
        $meta       = wp_get_attachment_metadata( $attachment_id );
        $upload     = wp_get_upload_dir();
        $orig_w     = isset( $meta['width'] )  ? (int) $meta['width']  : 0;
        $orig_h     = isset( $meta['height'] ) ? (int) $meta['height'] : 0;
        $orig_file  = get_attached_file( $attachment_id );
        $orig_fname = $orig_file ? basename( $orig_file ) : '';
        $subdir     = isset( $meta['file'] ) ? dirname( $meta['file'] ) : '';
        $base_url   = rtrim( $upload['baseurl'], '/' ) . ( $subdir ? '/' . $subdir : '' );
        $base_dir   = rtrim( $upload['basedir'], '/' ) . ( $subdir ? '/' . $subdir : '' );

        // --- Crop / focal meta ---
        $crop_mode = get_post_meta( $attachment_id, WPIR_Crop_Editor::META_CROP_MODE, true ) ?: 'center';
        $focal_x   = get_post_meta( $attachment_id, WPIR_Crop_Editor::META_FOCAL_X, true ) ?: '0.5';
        $focal_y   = get_post_meta( $attachment_id, WPIR_Crop_Editor::META_FOCAL_Y, true ) ?: '0.5';

        $size_focals = array();
        foreach ( WPIR_Crop_Editor::get_image_sizes_for_js() as $size ) {
            $raw = get_post_meta( $attachment_id, WPIR_Crop_Editor::META_SIZE_FOCAL_PREFIX . $size['name'], true );
            if ( $raw ) {
                $data = json_decode( $raw, true );
                if ( $data && isset( $data['x'], $data['y'] ) ) {
                    $size_focals[ $size['name'] ] = $data;
                }
            }
        }

        // --- Image URLs for the JS editor ---
        $preview_url = wp_get_attachment_image_url( $attachment_id, 'medium' )
            ?: wp_get_attachment_image_url( $attachment_id, 'full' );
        $full_url    = wp_get_attachment_image_url( $attachment_id, 'large' )
            ?: wp_get_attachment_image_url( $attachment_id, 'full' );

        // --- Registered sizes (for the on-disk table) ---
        $registered = function_exists( 'wp_get_registered_image_subsizes' )
            ? wp_get_registered_image_subsizes()
            : array();

        $generated = isset( $meta['sizes'] ) ? $meta['sizes'] : array();

        // --- Nonces ---
        $regen_nonce = wp_create_nonce( 'wpir_regenerate' );

        ?>
        <div class="wrap" id="wpir-crop-page">

            <p>
                <a href="<?php echo esc_url( admin_url( 'upload.php' ) ); ?>">&larr; <?php esc_html_e( 'Back to Media Library', 'wp-image-resizer' ); ?></a>
            </p>

            <h1>
                <?php
                echo esc_html(
                    sprintf(
                        /* translators: %s: image title */
                        __( '"%s" — Manage Crops', 'wp-image-resizer' ),
                        get_the_title( $attachment_id )
                    )
                );
                ?>
            </h1>

            <?php if ( $orig_w && $orig_h ) : ?>
                <p class="description">
                    <?php
                    echo esc_html(
                        sprintf(
                            /* translators: 1: filename 2: width 3: height */
                            __( 'Original: %1$s | %2$d × %3$d px', 'wp-image-resizer' ),
                            $orig_fname,
                            $orig_w,
                            $orig_h
                        )
                    );
                    ?>
                </p>
            <?php endif; ?>

            <hr />

            <?php if ( $preview_url ) : ?>

                <!-- ================================================================
                     Crop Settings — JS-rendered via WPIR.initFocalPointEditor()
                ================================================================ -->
                <h2><?php esc_html_e( 'Crop Settings', 'wp-image-resizer' ); ?></h2>
                <p class="description">
                    <?php esc_html_e( 'Set the crop mode and focal point applied when generating new thumbnails.', 'wp-image-resizer' ); ?>
                </p>
                <div id="wpir-crop-editor-wrap"></div>

            <?php endif; ?>

            <hr />

            <!-- ================================================================
                 Thumbnails on Disk — PHP-rendered from attachment metadata
            ================================================================ -->
            <h2><?php esc_html_e( 'Thumbnails on Disk', 'wp-image-resizer' ); ?></h2>
            <p class="description">
                <?php esc_html_e( 'These are the actual image files currently stored on disk. Click "Regenerate All Thumbnails" after changing crop settings to update them.', 'wp-image-resizer' ); ?>
            </p>

            <table class="wp-list-table widefat fixed striped" id="wpir-thumbs-table">
                <thead>
                    <tr>
                        <th><?php esc_html_e( 'Size', 'wp-image-resizer' ); ?></th>
                        <th><?php esc_html_e( 'Dimensions', 'wp-image-resizer' ); ?></th>
                        <th><?php esc_html_e( 'Type', 'wp-image-resizer' ); ?></th>
                        <th><?php esc_html_e( 'File', 'wp-image-resizer' ); ?></th>
                        <th><?php esc_html_e( 'Thumbnail', 'wp-image-resizer' ); ?></th>
                        <th><?php esc_html_e( 'Actions', 'wp-image-resizer' ); ?></th>
                        <th><?php esc_html_e( 'Override', 'wp-image-resizer' ); ?></th>
                    </tr>
                </thead>
                <tbody>
                    <?php
                    // Full / original is always on disk.
                    ?>
                    <tr>
                        <td><strong><?php esc_html_e( 'full (original)', 'wp-image-resizer' ); ?></strong></td>
                        <td><?php echo esc_html( $orig_w . ' × ' . $orig_h ); ?></td>
                        <td><em><?php esc_html_e( 'original', 'wp-image-resizer' ); ?></em></td>
                        <td><?php echo esc_html( $orig_fname ); ?></td>
                        <td>
                            <?php if ( $full_url ) : ?>
                                <a href="<?php echo esc_url( $full_url ); ?>" target="_blank">
                                    <img src="<?php echo esc_url( $full_url ); ?>"
                                         style="max-width:80px;max-height:80px;border:1px solid #ddd;"
                                         alt="" />
                                </a>
                            <?php endif; ?>
                        </td>
                        <td>—</td>
                        <td>—</td>
                    </tr>
                    <?php
                    foreach ( $registered as $size_name => $size_data ) :
                        $is_crop   = ! empty( $size_data['crop'] );
                        if ( ! $is_crop ) {
                            continue;
                        }
                        $reg_w     = isset( $size_data['width'] )  ? (int) $size_data['width']  : 0;
                        $reg_h     = isset( $size_data['height'] ) ? (int) $size_data['height'] : 0;
                        $gen       = isset( $generated[ $size_name ] ) ? $generated[ $size_name ] : null;
                        $gen_file  = $gen ? $gen['file'] : null;
                        $gen_w     = $gen ? (int) $gen['width']  : $reg_w;
                        $gen_h     = $gen ? (int) $gen['height'] : $reg_h;
                        $file_path = $gen_file ? ( $base_dir . '/' . $gen_file ) : null;
                        $on_disk   = $file_path && file_exists( $file_path );
                        $thumb_url = $gen_file ? ( $base_url . '/' . $gen_file ) : null;
                        $has_override = (bool) get_post_meta(
                            $attachment_id,
                            WPIR_Crop_Editor::META_SIZE_FOCAL_PREFIX . $size_name,
                            true
                        );
                        $label = ucwords( str_replace( array( '-', '_' ), ' ', $size_name ) );
                    ?>
                    <tr>
                        <td><strong><?php echo esc_html( $label ); ?></strong></td>
                        <td><?php echo esc_html( $gen_w . ' × ' . $gen_h ); ?></td>
                        <td>
                            <?php if ( $is_crop ) : ?>
                                <span style="color:#0073aa;"><?php esc_html_e( 'crop', 'wp-image-resizer' ); ?></span>
                            <?php else : ?>
                                <span style="color:#666;"><?php esc_html_e( 'scale-to-fit', 'wp-image-resizer' ); ?></span>
                            <?php endif; ?>
                        </td>
                        <td>
                            <?php if ( $gen_file ) : ?>
                                <?php echo esc_html( $gen_file ); ?>
                            <?php else : ?>
                                <em style="color:#999;"><?php esc_html_e( 'not generated', 'wp-image-resizer' ); ?></em>
                            <?php endif; ?>
                        </td>
                        <td>
                            <?php if ( $on_disk && $thumb_url ) : ?>
                                <a href="<?php echo esc_url( $thumb_url ); ?>" target="_blank">
                                    <img src="<?php echo esc_url( $thumb_url ); ?>"
                                         style="max-width:80px;height:auto;display:block;"
                                         alt="" />
                                </a>
                            <?php elseif ( $gen_file ) : ?>
                                <span style="color:#cc0000;" title="<?php esc_attr_e( 'File not found on disk', 'wp-image-resizer' ); ?>">
                                    &#9888; <?php esc_html_e( 'Missing', 'wp-image-resizer' ); ?>
                                </span>
                            <?php else : ?>
                                <span style="color:#999;">—</span>
                            <?php endif; ?>
                        </td>
                        <td>
                            <?php if ( $is_crop ) : ?>
                                <div style="display:flex;flex-direction:column;gap:4px;align-items:flex-start;">
                                    <button type="button" class="button button-small wpir-table-adjust-btn"
                                            data-size="<?php echo esc_attr( $size_name ); ?>">
                                        <?php esc_html_e( 'Adjust Crop', 'wp-image-resizer' ); ?>
                                    </button>
                                    <button type="button" class="button button-small wpir-table-regen-btn"
                                            data-size="<?php echo esc_attr( $size_name ); ?>">
                                        <?php esc_html_e( 'Regenerate', 'wp-image-resizer' ); ?>
                                    </button>
                                </div>
                            <?php else : ?>
                                —
                            <?php endif; ?>
                        </td>
                        <td>
                            <?php if ( $has_override ) : ?>
                                <span style="background:#0073aa;color:#fff;font-size:10px;font-weight:600;padding:2px 6px;border-radius:3px;">
                                    <?php esc_html_e( 'Custom', 'wp-image-resizer' ); ?>
                                </span>
                            <?php else : ?>
                                —
                            <?php endif; ?>
                        </td>
                    </tr>
                    <?php endforeach; ?>
                </tbody>
            </table>

            <p style="margin-top:16px;">
                <button type="button" id="wpir-regen-btn" class="button button-primary">
                    <?php esc_html_e( 'Regenerate All Thumbnails', 'wp-image-resizer' ); ?>
                </button>
                <span id="wpir-regen-msg" style="margin-left:12px;font-size:13px;display:none;"></span>
            </p>

        </div><!-- .wrap -->

        <?php if ( $preview_url ) : ?>
        <script>
        function wpirOpenAdjust(sizeName) {
            if (typeof WPIR === 'undefined' || typeof WPIR.openCropToolForSize !== 'function') return;
            WPIR.openCropToolForSize(sizeName);
        }

        function wpirRegenAttachment(triggerBtn) {
            var origLabel = triggerBtn ? triggerBtn.textContent : '';
            if (triggerBtn) {
                triggerBtn.disabled    = true;
                triggerBtn.textContent = <?php echo wp_json_encode( __( 'Regenerating…', 'wp-image-resizer' ) ); ?>;
            }
            var fd = new FormData();
            fd.append('action',        'wpir_regenerate_single');
            fd.append('nonce',         <?php echo wp_json_encode( $regen_nonce ); ?>);
            fd.append('attachment_id', <?php echo (int) $attachment_id; ?>);

            fetch(ajaxurl, { method: 'POST', body: fd })
                .then(function (r) { return r.json(); })
                .then(function (resp) {
                    if (resp.success) {
                        window.location.reload();
                    } else {
                        alert(resp.data || <?php echo wp_json_encode( __( 'Error.', 'wp-image-resizer' ) ); ?>);
                        if (triggerBtn) {
                            triggerBtn.disabled    = false;
                            triggerBtn.textContent = origLabel;
                        }
                    }
                })
                .catch(function (err) {
                    alert('Network error: ' + err.message);
                    if (triggerBtn) {
                        triggerBtn.disabled    = false;
                        triggerBtn.textContent = origLabel;
                    }
                });
        }

        document.addEventListener('DOMContentLoaded', function () {
            // Wire up "Adjust Crop" buttons in the Thumbnails table.
            document.querySelectorAll('.wpir-table-adjust-btn').forEach(function (btn) {
                btn.addEventListener('click', function () {
                    wpirOpenAdjust(this.dataset.size);
                });
            });

            // Wire up per-row "Regenerate" buttons.
            document.querySelectorAll('.wpir-table-regen-btn').forEach(function (btn) {
                btn.addEventListener('click', function () {
                    wpirRegenAttachment(this);
                });
            });

            if (typeof WPIR === 'undefined' || typeof WPIR.initFocalPointEditor !== 'function') {
                return;
            }
            WPIR.initFocalPointEditor(
                document.getElementById('wpir-crop-editor-wrap'),
                <?php echo (int) $attachment_id; ?>,
                <?php echo wp_json_encode( $preview_url ); ?>,
                <?php echo wp_json_encode( $focal_x ); ?>,
                <?php echo wp_json_encode( $focal_y ); ?>,
                <?php echo wp_json_encode( $crop_mode ); ?>,
                <?php echo (int) $orig_w; ?>,
                <?php echo (int) $orig_h; ?>,
                <?php echo wp_json_encode( (object) $size_focals ); ?>,
                <?php echo wp_json_encode( $full_url ); ?>
            );
        });

        // Regenerate All button.
        (function () {
            var btn   = document.getElementById('wpir-regen-btn');
            var msg   = document.getElementById('wpir-regen-msg');
            if (!btn) return;

            btn.addEventListener('click', function () {
                btn.disabled    = true;
                msg.style.display = 'inline';
                msg.style.color   = '#666';
                msg.textContent   = <?php echo wp_json_encode( __( 'Regenerating…', 'wp-image-resizer' ) ); ?>;

                var fd = new FormData();
                fd.append('action',        'wpir_regenerate_single');
                fd.append('nonce',         <?php echo wp_json_encode( $regen_nonce ); ?>);
                fd.append('attachment_id', <?php echo (int) $attachment_id; ?>);

                fetch(ajaxurl, { method: 'POST', body: fd })
                    .then(function (r) { return r.json(); })
                    .then(function (resp) {
                        if (resp.success) {
                            msg.style.color = '#00a32a';
                            msg.textContent = <?php echo wp_json_encode( __( 'Done! Reloading…', 'wp-image-resizer' ) ); ?>;
                            setTimeout(function () { window.location.reload(); }, 800);
                        } else {
                            msg.style.color = '#cc0000';
                            msg.textContent = (resp.data || <?php echo wp_json_encode( __( 'Error.', 'wp-image-resizer' ) ); ?>);
                            btn.disabled    = false;
                        }
                    })
                    .catch(function (err) {
                        msg.style.color = '#cc0000';
                        msg.textContent = 'Network error: ' + err.message;
                        btn.disabled    = false;
                    });
            });
        })();
        </script>
        <?php endif; ?>
        <?php
    }
}
