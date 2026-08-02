variable "bucket_name" {
  description = "Name of the private data bucket."
  type        = string
}

variable "abort_incomplete_multipart_upload_days" {
  description = "Number of days after which S3 aborts incomplete multipart uploads. This does not expire completed objects."
  type        = number
  default     = 7

  validation {
    condition     = var.abort_incomplete_multipart_upload_days >= 1
    error_message = "abort_incomplete_multipart_upload_days must be at least 1."
  }
}
