output "id" {
  description = "Data bucket name."
  value       = aws_s3_bucket.this.id
}

output "arn" {
  description = "Data bucket ARN."
  value       = aws_s3_bucket.this.arn
}

output "regional_domain_name" {
  description = "Regional S3 domain used by CloudFront."
  value       = aws_s3_bucket.this.bucket_regional_domain_name
}
