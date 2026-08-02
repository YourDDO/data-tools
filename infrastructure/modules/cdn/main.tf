locals {
  origin_id = "s3-${var.bucket_id}"
}

data "aws_caller_identity" "current" {}

data "aws_partition" "current" {}

resource "aws_cloudfront_origin_access_control" "this" {
  name                              = "${var.bucket_id}-oac"
  description                       = "Access from the YourDDO data CDN to its private S3 origin"
  origin_access_control_origin_type = "s3"
  signing_behavior                  = "always"
  signing_protocol                  = "sigv4"
}

resource "aws_cloudfront_cache_policy" "data" {
  name        = "${var.bucket_id}-immutable-releases"
  comment     = "Cache content-addressed release objects for one year"
  default_ttl = 31536000
  max_ttl     = 31536000
  min_ttl     = 31536000

  parameters_in_cache_key_and_forwarded_to_origin {
    cookies_config {
      cookie_behavior = "none"
    }

    headers_config {
      header_behavior = "none"
    }

    query_strings_config {
      query_string_behavior = "none"
    }

    enable_accept_encoding_brotli = true
    enable_accept_encoding_gzip   = true
  }
}

resource "aws_cloudfront_cache_policy" "latest" {
  name        = "${var.bucket_id}-latest"
  comment     = "Do not cache the mutable latest.json release pointer"
  default_ttl = 0
  max_ttl     = 0
  min_ttl     = 0

  parameters_in_cache_key_and_forwarded_to_origin {
    cookies_config {
      cookie_behavior = "none"
    }

    headers_config {
      header_behavior = "none"
    }

    query_strings_config {
      query_string_behavior = "none"
    }

    enable_accept_encoding_brotli = false
    enable_accept_encoding_gzip   = false
  }
}

resource "aws_cloudfront_distribution" "this" {
  aliases         = [var.domain_name]
  enabled         = true
  http_version    = "http2and3"
  is_ipv6_enabled = true
  price_class     = var.price_class
  web_acl_id      = var.web_acl_id

  origin {
    domain_name              = var.bucket_regional_domain_name
    origin_access_control_id = aws_cloudfront_origin_access_control.this.id
    origin_id                = local.origin_id
  }

  default_cache_behavior {
    allowed_methods            = ["GET", "HEAD"]
    cache_policy_id            = aws_cloudfront_cache_policy.data.id
    cached_methods             = ["GET", "HEAD"]
    compress                   = true
    response_headers_policy_id = "60669652-455b-4ae9-85a4-c4c02393f86c"
    target_origin_id           = local.origin_id
    viewer_protocol_policy     = "redirect-to-https"
  }

  ordered_cache_behavior {
    path_pattern               = "latest.json"
    allowed_methods            = ["GET", "HEAD"]
    cache_policy_id            = aws_cloudfront_cache_policy.latest.id
    cached_methods             = ["GET", "HEAD"]
    compress                   = true
    response_headers_policy_id = "60669652-455b-4ae9-85a4-c4c02393f86c"
    target_origin_id           = local.origin_id
    viewer_protocol_policy     = "redirect-to-https"
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    acm_certificate_arn      = var.acm_certificate_arn
    minimum_protocol_version = "TLSv1.2_2025"
    ssl_support_method       = "sni-only"
  }
}

resource "aws_s3_bucket_policy" "origin" {
  bucket = var.bucket_id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Sid       = "DenyInsecureTransport"
        Effect    = "Deny"
        Action    = "s3:*"
        Resource  = [var.bucket_arn, "${var.bucket_arn}/*"]
        Principal = "*"
        Condition = {
          Bool = {
            "aws:SecureTransport" = "false"
          }
        }
      },
      {
        Sid      = "AllowCloudFrontReadOnly"
        Effect   = "Allow"
        Action   = "s3:GetObject"
        Resource = "${var.bucket_arn}/*"
        Principal = {
          Service = "cloudfront.amazonaws.com"
        }
        Condition = {
          StringEquals = {
            "AWS:SourceArn" = "arn:${data.aws_partition.current.partition}:cloudfront::${data.aws_caller_identity.current.account_id}:distribution/${aws_cloudfront_distribution.this.id}"
          }
        }
      },
    ]
  })
}
