data "aws_caller_identity" "current" {}

data "aws_vpc" "shared" {
  id = var.vpc_id
}

data "aws_subnet" "private" {
  for_each = toset(var.private_subnet_ids)
  id       = each.value
}

data "aws_route53_zone" "shared" {
  zone_id = var.route53_zone_id
}

data "aws_security_group" "compendium" {
  id = var.compendium_security_group_id
}

check "private_subnets_belong_to_shared_vpc" {
  assert {
    condition     = alltrue([for subnet in data.aws_subnet.private : subnet.vpc_id == data.aws_vpc.shared.id])
    error_message = "Every private_subnet_id must belong to vpc_id."
  }
}

check "compendium_belongs_to_shared_vpc" {
  assert {
    condition     = data.aws_security_group.compendium.vpc_id == data.aws_vpc.shared.id
    error_message = "compendium_security_group_id must belong to vpc_id."
  }
}

check "cdn_domain_belongs_to_hosted_zone" {
  assert {
    condition = !data.aws_route53_zone.shared.private_zone && (
      trimsuffix(var.cdn_domain_name, ".") == trimsuffix(data.aws_route53_zone.shared.name, ".") ||
      endswith(trimsuffix(var.cdn_domain_name, "."), ".${trimsuffix(data.aws_route53_zone.shared.name, ".")}")
    )
    error_message = "cdn_domain_name must belong to the existing public Route 53 hosted zone."
  }
}
