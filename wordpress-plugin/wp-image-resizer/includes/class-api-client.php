<?php
/**
 * API client for communicating with the image-service.
 */

if ( ! defined( 'ABSPATH' ) ) {
    exit;
}

class WPIR_API_Client {

    /** @var string Service base URL. */
    private $service_url;

    /** @var string JWT shared secret. */
    private $jwt_secret;

    public function __construct() {
        $this->service_url = get_option( 'wpir_service_url', '' );
        $this->jwt_secret  = get_option( 'wpir_jwt_secret', '' );
    }

    /**
     * Resize an image via the service.
     *
     * @param string $file_path Local path to the image file.
     * @param int    $width     Target width.
     * @param int    $height    Target height.
     * @param string $crop      Crop mode: 'center', 'smart', or 'x,y'.
     * @param string $format    Output format: jpeg, png, webp, avif.
     * @param int    $quality   Quality/compression (0 = use server default).
     * @return array|WP_Error   Array with 'data' (binary) and 'content_type', or WP_Error.
     */
    public function resize( $file_path, $width, $height, $crop = 'center', $format = null, $quality = 0 ) {
        if ( empty( $this->service_url ) || empty( $this->jwt_secret ) ) {
            return new WP_Error( 'wpir_not_configured', __( 'Image Resizer is not configured.', 'wp-image-resizer' ) );
        }

        $token = $this->generate_jwt();
        if ( is_wp_error( $token ) ) {
            return $token;
        }

        if ( null === $format ) {
            $format = get_option( 'wpir_default_format', 'jpeg' );
        }

        if ( 0 === $quality ) {
            $quality = $this->get_quality_for_format( $format );
        }

        $boundary = wp_generate_password( 24, false );
        $body     = $this->build_multipart_body( $boundary, $file_path, $width, $height, $crop, $format, $quality );

        if ( is_wp_error( $body ) ) {
            return $body;
        }

        $response = wp_remote_post(
            $this->service_url . '/resize',
            array(
                'timeout'   => 60,
                'headers'   => array(
                    'Authorization' => 'Bearer ' . $token,
                    'Content-Type'  => 'multipart/form-data; boundary=' . $boundary,
                ),
                'body'      => $body,
                'sslverify' => apply_filters( 'wpir_sslverify', true ),
            )
        );

        if ( is_wp_error( $response ) ) {
            return new WP_Error(
                'wpir_request_failed',
                sprintf( __( 'Image service request failed: %s', 'wp-image-resizer' ), $response->get_error_message() )
            );
        }

        $code = wp_remote_retrieve_response_code( $response );
        if ( 200 !== $code ) {
            $error_body = wp_remote_retrieve_body( $response );
            $error_data = json_decode( $error_body, true );
            $error_msg  = isset( $error_data['error'] ) ? $error_data['error'] : "HTTP $code";

            return new WP_Error(
                'wpir_service_error',
                sprintf( __( 'Image service error (%d): %s', 'wp-image-resizer' ), $code, $error_msg )
            );
        }

        return array(
            'data'         => wp_remote_retrieve_body( $response ),
            'content_type' => wp_remote_retrieve_header( $response, 'content-type' ),
        );
    }

    /**
     * Resize an image from URL via the service.
     *
     * @param string $url       Image URL.
     * @param int    $width     Target width.
     * @param int    $height    Target height.
     * @param string $crop      Crop mode.
     * @param string $format    Output format.
     * @param int    $quality   Quality (0 = server default).
     * @return array|WP_Error
     */
    public function resize_from_url( $url, $width, $height, $crop = 'center', $format = null, $quality = 0 ) {
        if ( empty( $this->service_url ) || empty( $this->jwt_secret ) ) {
            return new WP_Error( 'wpir_not_configured', __( 'Image Resizer is not configured.', 'wp-image-resizer' ) );
        }

        $token = $this->generate_jwt();
        if ( is_wp_error( $token ) ) {
            return $token;
        }

        if ( null === $format ) {
            $format = get_option( 'wpir_default_format', 'jpeg' );
        }

        if ( 0 === $quality ) {
            $quality = $this->get_quality_for_format( $format );
        }

        $boundary = wp_generate_password( 24, false );
        $body     = $this->build_multipart_body_url( $boundary, $url, $width, $height, $crop, $format, $quality );

        $response = wp_remote_post(
            $this->service_url . '/resize',
            array(
                'timeout'   => 60,
                'headers'   => array(
                    'Authorization' => 'Bearer ' . $token,
                    'Content-Type'  => 'multipart/form-data; boundary=' . $boundary,
                ),
                'body'      => $body,
                'sslverify' => apply_filters( 'wpir_sslverify', true ),
            )
        );

        if ( is_wp_error( $response ) ) {
            return new WP_Error(
                'wpir_request_failed',
                sprintf( __( 'Image service request failed: %s', 'wp-image-resizer' ), $response->get_error_message() )
            );
        }

        $code = wp_remote_retrieve_response_code( $response );
        if ( 200 !== $code ) {
            $error_body = wp_remote_retrieve_body( $response );
            $error_data = json_decode( $error_body, true );
            $error_msg  = isset( $error_data['error'] ) ? $error_data['error'] : "HTTP $code";

            return new WP_Error(
                'wpir_service_error',
                sprintf( __( 'Image service error (%d): %s', 'wp-image-resizer' ), $code, $error_msg )
            );
        }

        return array(
            'data'         => wp_remote_retrieve_body( $response ),
            'content_type' => wp_remote_retrieve_header( $response, 'content-type' ),
        );
    }

    /**
     * Check if the service is reachable.
     *
     * @return bool
     */
    public function health_check() {
        if ( empty( $this->service_url ) ) {
            return false;
        }

        $response = wp_remote_get(
            $this->service_url . '/health',
            array( 'timeout' => 5 )
        );

        if ( is_wp_error( $response ) ) {
            return false;
        }

        return 200 === wp_remote_retrieve_response_code( $response );
    }

    /**
     * Generate a JWT token.
     *
     * @return string|WP_Error
     */
    private function generate_jwt() {
        $header = array(
            'alg' => 'HS256',
            'typ' => 'JWT',
        );

        $now     = time();
        $payload = array(
            'iss' => 'image-service',
            'iat' => $now,
            'exp' => $now + 300, // 5 minutes.
        );

        $header_b64  = $this->base64url_encode( wp_json_encode( $header ) );
        $payload_b64 = $this->base64url_encode( wp_json_encode( $payload ) );
        $signature   = hash_hmac( 'sha256', $header_b64 . '.' . $payload_b64, $this->jwt_secret, true );
        $sig_b64     = $this->base64url_encode( $signature );

        return $header_b64 . '.' . $payload_b64 . '.' . $sig_b64;
    }

    /**
     * Base64url encode (no padding).
     */
    private function base64url_encode( $data ) {
        return rtrim( strtr( base64_encode( $data ), '+/', '-_' ), '=' );
    }

    /**
     * Get quality setting for the given format.
     */
    private function get_quality_for_format( $format ) {
        $key = 'wpir_quality_' . $format;
        return (int) get_option( $key, 0 );
    }

    /**
     * Build multipart form body with file upload.
     */
    private function build_multipart_body( $boundary, $file_path, $width, $height, $crop, $format, $quality ) {
        if ( ! file_exists( $file_path ) || ! is_readable( $file_path ) ) {
            return new WP_Error( 'wpir_file_error', __( 'Image file not found or not readable.', 'wp-image-resizer' ) );
        }

        $file_data = file_get_contents( $file_path );
        $filename  = basename( $file_path );
        $mime      = wp_check_filetype( $file_path )['type'] ?? 'application/octet-stream';

        $body = '';
        $body .= $this->multipart_field( $boundary, 'width', (string) $width );
        $body .= $this->multipart_field( $boundary, 'height', (string) $height );
        $body .= $this->multipart_field( $boundary, 'crop', $crop );
        $body .= $this->multipart_field( $boundary, 'format', $format );

        if ( $quality > 0 ) {
            $body .= $this->multipart_field( $boundary, 'quality', (string) $quality );
        }

        // File field.
        $body .= "--{$boundary}\r\n";
        $body .= "Content-Disposition: form-data; name=\"image\"; filename=\"{$filename}\"\r\n";
        $body .= "Content-Type: {$mime}\r\n\r\n";
        $body .= $file_data . "\r\n";
        $body .= "--{$boundary}--\r\n";

        return $body;
    }

    /**
     * Build multipart form body with URL parameter.
     */
    private function build_multipart_body_url( $boundary, $url, $width, $height, $crop, $format, $quality ) {
        $body = '';
        $body .= $this->multipart_field( $boundary, 'url', $url );
        $body .= $this->multipart_field( $boundary, 'width', (string) $width );
        $body .= $this->multipart_field( $boundary, 'height', (string) $height );
        $body .= $this->multipart_field( $boundary, 'crop', $crop );
        $body .= $this->multipart_field( $boundary, 'format', $format );

        if ( $quality > 0 ) {
            $body .= $this->multipart_field( $boundary, 'quality', (string) $quality );
        }

        $body .= "--{$boundary}--\r\n";
        return $body;
    }

    /**
     * Build a single multipart form field.
     */
    private function multipart_field( $boundary, $name, $value ) {
        return "--{$boundary}\r\n" .
               "Content-Disposition: form-data; name=\"{$name}\"\r\n\r\n" .
               $value . "\r\n";
    }
}
