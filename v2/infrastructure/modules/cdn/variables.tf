variable "bucket_id" {
  description = "Name of the private S3 origin bucket."
  type        = string
}

variable "bucket_arn" {
  description = "ARN of the private S3 origin bucket."
  type        = string
}

variable "bucket_regional_domain_name" {
  description = "Regional domain name of the S3 origin bucket."
  type        = string
}

variable "domain_name" {
  description = "Custom DNS alias served by CloudFront."
  type        = string
}

variable "acm_certificate_arn" {
  description = "ARN of an existing ACM certificate in us-east-2."
  type        = string
}

variable "price_class" {
  description = "CloudFront price class."
  type        = string
  default     = "PriceClass_100"
}

variable "web_acl_id" {
  description = "Optional existing AWS WAF web ACL ARN."
  type        = string
  default     = null
  nullable    = true
}
