output "schedule_arn" {
  description = "EventBridge Scheduler schedule ARN."
  value       = aws_scheduler_schedule.this.arn
}
