<?php
/**
 * Plugin Name: WP Image Resizer
 * Plugin URI:  https://github.com/jlentink/image-service
 * Description: Offloads image resizing and cropping to an external image-service instance. Supports JPEG, PNG, WebP, and AVIF with smart crop and focal point crop.
 * Version:     0.0.1
 * Author:      jlentink
 * License:     GPL-2.0-or-later
 * Text Domain: wp-image-resizer
 * Requires PHP: 7.4
 * Requires at least: 5.8
 */

if ( ! defined( 'ABSPATH' ) ) {
    exit;
}

define( 'WPIR_VERSION', '0.0.1' );
define( 'WPIR_PLUGIN_DIR', plugin_dir_path( __FILE__ ) );
define( 'WPIR_PLUGIN_URL', plugin_dir_url( __FILE__ ) );
define( 'WPIR_PLUGIN_BASENAME', plugin_basename( __FILE__ ) );

// Autoload includes.
require_once WPIR_PLUGIN_DIR . 'includes/class-settings.php';
require_once WPIR_PLUGIN_DIR . 'includes/class-api-client.php';
require_once WPIR_PLUGIN_DIR . 'includes/class-image-editor.php';
require_once WPIR_PLUGIN_DIR . 'includes/class-media-handler.php';
require_once WPIR_PLUGIN_DIR . 'includes/class-crop-editor.php';

/**
 * Initialize the plugin.
 */
function wpir_init() {
    // Always load settings page in admin.
    if ( is_admin() ) {
        new WPIR_Settings();
    }

    // Only hook into image processing if configured.
    if ( wpir_is_configured() ) {
        new WPIR_Media_Handler();
        new WPIR_Crop_Editor();
    }
}
add_action( 'plugins_loaded', 'wpir_init' );

/**
 * Activation hook.
 */
function wpir_activate() {
    // Set default options.
    $defaults = array(
        'wpir_service_url'   => '',
        'wpir_jwt_secret'    => '',
        'wpir_default_format' => 'jpeg',
        'wpir_enabled'       => '0',
        'wpir_quality_jpeg'  => '85',
        'wpir_quality_webp'  => '80',
        'wpir_quality_avif'  => '60',
        'wpir_quality_png'   => '6',
    );

    foreach ( $defaults as $key => $value ) {
        if ( false === get_option( $key ) ) {
            add_option( $key, $value );
        }
    }
}
register_activation_hook( __FILE__, 'wpir_activate' );

/**
 * Deactivation hook.
 */
function wpir_deactivate() {
    // Remove the image editor override filter so WordPress falls back to default.
    remove_filter( 'wp_image_editors', 'wpir_register_image_editor' );
}
register_deactivation_hook( __FILE__, 'wpir_deactivate' );

/**
 * Check if the plugin is fully configured and enabled.
 */
function wpir_is_configured() {
    $url    = get_option( 'wpir_service_url', '' );
    $secret = get_option( 'wpir_jwt_secret', '' );
    $enabled = get_option( 'wpir_enabled', '0' );

    return $enabled === '1' && ! empty( $url ) && ! empty( $secret );
}

/**
 * Add settings link on plugin page.
 */
function wpir_plugin_action_links( $links ) {
    $settings_link = sprintf(
        '<a href="%s">%s</a>',
        admin_url( 'options-general.php?page=wp-image-resizer' ),
        __( 'Settings', 'wp-image-resizer' )
    );
    array_unshift( $links, $settings_link );
    return $links;
}
add_filter( 'plugin_action_links_' . WPIR_PLUGIN_BASENAME, 'wpir_plugin_action_links' );
