<?php
/**
 * Plugin settings page.
 */

if ( ! defined( 'ABSPATH' ) ) {
    exit;
}

class WPIR_Settings {

    public function __construct() {
        add_action( 'admin_menu', array( $this, 'add_settings_page' ) );
        add_action( 'admin_init', array( $this, 'register_settings' ) );
    }

    /**
     * Add the settings page under Settings menu.
     */
    public function add_settings_page() {
        add_options_page(
            __( 'WP Image Resizer', 'wp-image-resizer' ),
            __( 'Image Resizer', 'wp-image-resizer' ),
            'manage_options',
            'wp-image-resizer',
            array( $this, 'render_settings_page' )
        );
    }

    /**
     * Register all settings.
     */
    public function register_settings() {
        // Connection section.
        add_settings_section(
            'wpir_connection',
            __( 'Service Connection', 'wp-image-resizer' ),
            array( $this, 'render_connection_section' ),
            'wp-image-resizer'
        );

        $this->add_field( 'wpir_enabled', __( 'Enable', 'wp-image-resizer' ), 'render_checkbox_field', 'wpir_connection' );
        $this->add_field( 'wpir_service_url', __( 'Service URL', 'wp-image-resizer' ), 'render_url_field', 'wpir_connection' );
        $this->add_field( 'wpir_jwt_secret', __( 'JWT Secret', 'wp-image-resizer' ), 'render_secret_field', 'wpir_connection' );

        // Quality section.
        add_settings_section(
            'wpir_quality',
            __( 'Image Quality', 'wp-image-resizer' ),
            array( $this, 'render_quality_section' ),
            'wp-image-resizer'
        );

        $this->add_field( 'wpir_default_format', __( 'Default Format', 'wp-image-resizer' ), 'render_format_field', 'wpir_quality' );
        $this->add_field( 'wpir_quality_jpeg', __( 'JPEG Quality (1-100)', 'wp-image-resizer' ), 'render_number_field', 'wpir_quality' );
        $this->add_field( 'wpir_quality_webp', __( 'WebP Quality (1-100)', 'wp-image-resizer' ), 'render_number_field', 'wpir_quality' );
        $this->add_field( 'wpir_quality_avif', __( 'AVIF Quality (1-100)', 'wp-image-resizer' ), 'render_number_field', 'wpir_quality' );
        $this->add_field( 'wpir_quality_png', __( 'PNG Compression (0-9)', 'wp-image-resizer' ), 'render_number_field', 'wpir_quality' );
    }

    private function add_field( $id, $title, $callback, $section ) {
        register_setting( 'wpir_settings', $id, array( 'sanitize_callback' => array( $this, 'sanitize_' . $id ) ) );
        add_settings_field( $id, $title, array( $this, $callback ), 'wp-image-resizer', $section, array( 'id' => $id ) );
    }

    // --- Render callbacks ---

    public function render_connection_section() {
        echo '<p>' . esc_html__( 'Configure the connection to your image-service instance.', 'wp-image-resizer' ) . '</p>';
    }

    public function render_quality_section() {
        echo '<p>' . esc_html__( 'Set default quality/compression for each output format.', 'wp-image-resizer' ) . '</p>';
    }

    public function render_checkbox_field( $args ) {
        $value = get_option( $args['id'], '0' );
        printf(
            '<input type="checkbox" id="%s" name="%s" value="1" %s />',
            esc_attr( $args['id'] ),
            esc_attr( $args['id'] ),
            checked( $value, '1', false )
        );

        if ( $args['id'] === 'wpir_enabled' ) {
            echo '<p class="description">' . esc_html__( 'Enable to use the external image service for image resizing.', 'wp-image-resizer' ) . '</p>';

            // Show connection status.
            if ( $value === '1' && wpir_is_configured() ) {
                $client = new WPIR_API_Client();
                $status = $client->health_check();
                if ( $status ) {
                    echo '<p style="color:green;">' . esc_html__( 'Service is reachable.', 'wp-image-resizer' ) . '</p>';
                } else {
                    echo '<p style="color:red;">' . esc_html__( 'Service is not reachable. Check your URL.', 'wp-image-resizer' ) . '</p>';
                }
            }
        }
    }

    public function render_url_field( $args ) {
        $value = get_option( $args['id'], '' );
        printf(
            '<input type="url" id="%s" name="%s" value="%s" class="regular-text" placeholder="https://images.example.com" />',
            esc_attr( $args['id'] ),
            esc_attr( $args['id'] ),
            esc_attr( $value )
        );
        echo '<p class="description">' . esc_html__( 'Full URL to your image-service (e.g., https://images.example.com or http://localhost:8080).', 'wp-image-resizer' ) . '</p>';
    }

    public function render_secret_field( $args ) {
        $value = get_option( $args['id'], '' );
        printf(
            '<input type="password" id="%s" name="%s" value="%s" class="regular-text" autocomplete="off" />',
            esc_attr( $args['id'] ),
            esc_attr( $args['id'] ),
            esc_attr( $value )
        );
        echo '<p class="description">' . esc_html__( 'Shared JWT secret. Must match the auth.jwt_secret in your image-service config.', 'wp-image-resizer' ) . '</p>';
    }

    public function render_format_field( $args ) {
        $value = get_option( $args['id'], 'jpeg' );
        $formats = array(
            'jpeg' => 'JPEG',
            'png'  => 'PNG',
            'webp' => 'WebP',
            'avif' => 'AVIF',
        );
        echo '<select id="' . esc_attr( $args['id'] ) . '" name="' . esc_attr( $args['id'] ) . '">';
        foreach ( $formats as $key => $label ) {
            printf(
                '<option value="%s" %s>%s</option>',
                esc_attr( $key ),
                selected( $value, $key, false ),
                esc_html( $label )
            );
        }
        echo '</select>';
        echo '<p class="description">' . esc_html__( 'Default output format for resized images.', 'wp-image-resizer' ) . '</p>';
    }

    public function render_number_field( $args ) {
        $value = get_option( $args['id'], '' );
        $max = ( strpos( $args['id'], 'png' ) !== false ) ? 9 : 100;
        printf(
            '<input type="number" id="%s" name="%s" value="%s" min="0" max="%d" class="small-text" />',
            esc_attr( $args['id'] ),
            esc_attr( $args['id'] ),
            esc_attr( $value ),
            $max
        );
    }

    // --- Sanitize callbacks ---

    public function sanitize_wpir_enabled( $value ) {
        return $value === '1' ? '1' : '0';
    }

    public function sanitize_wpir_service_url( $value ) {
        $value = esc_url_raw( trim( $value ) );
        return rtrim( $value, '/' ); // Remove trailing slash.
    }

    public function sanitize_wpir_jwt_secret( $value ) {
        return sanitize_text_field( trim( $value ) );
    }

    public function sanitize_wpir_default_format( $value ) {
        $allowed = array( 'jpeg', 'png', 'webp', 'avif' );
        return in_array( $value, $allowed, true ) ? $value : 'jpeg';
    }

    public function sanitize_wpir_quality_jpeg( $value ) {
        return $this->sanitize_range( $value, 1, 100, 85 );
    }

    public function sanitize_wpir_quality_webp( $value ) {
        return $this->sanitize_range( $value, 1, 100, 80 );
    }

    public function sanitize_wpir_quality_avif( $value ) {
        return $this->sanitize_range( $value, 1, 100, 60 );
    }

    public function sanitize_wpir_quality_png( $value ) {
        return $this->sanitize_range( $value, 0, 9, 6 );
    }

    private function sanitize_range( $value, $min, $max, $default ) {
        $value = intval( $value );
        if ( $value < $min || $value > $max ) {
            return (string) $default;
        }
        return (string) $value;
    }

    /**
     * Render the settings page.
     */
    public function render_settings_page() {
        if ( ! current_user_can( 'manage_options' ) ) {
            return;
        }
        ?>
        <div class="wrap">
            <h1><?php echo esc_html( get_admin_page_title() ); ?></h1>
            <form method="post" action="options.php">
                <?php
                settings_fields( 'wpir_settings' );
                do_settings_sections( 'wp-image-resizer' );
                submit_button();
                ?>
            </form>
        </div>
        <?php
    }
}
