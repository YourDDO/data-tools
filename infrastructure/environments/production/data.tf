data "aws_caller_identity" "current" {}

data "aws_vpc" "shared" {
  id = var.vpc_id
}

data "aws_internet_gateway" "shared" {
  filter {
    name   = "attachment.vpc-id"
    values = [data.aws_vpc.shared.id]
  }
}

data "aws_subnet" "public" {
  id = var.public_subnet_id
}

data "aws_subnet" "private" {
  for_each = toset(var.private_subnet_ids)
  id       = each.value
}

data "aws_route_table" "public" {
  subnet_id = data.aws_subnet.public.id
}

locals {
  private_subnet_ids              = sort([for subnet in data.aws_subnet.private : subnet.id])
  private_subnet_ids_in_public_az = sort([for subnet in data.aws_subnet.private : subnet.id if subnet.availability_zone_id == data.aws_subnet.public.availability_zone_id])
  codebuild_private_subnet_id     = try(local.private_subnet_ids_in_public_az[0], local.private_subnet_ids[0])
  public_subnet_has_igw_default_route = anytrue([
    for route in data.aws_route_table.public.routes :
    route.cidr_block == "0.0.0.0/0" && route.gateway_id == data.aws_internet_gateway.shared.id
  ])
}

data "aws_route_table" "codebuild_private" {
  subnet_id = local.codebuild_private_subnet_id
}

locals {
  codebuild_subnet_has_igw_default_route = anytrue([
    for route in data.aws_route_table.codebuild_private.routes :
    route.cidr_block == "0.0.0.0/0" && route.gateway_id == data.aws_internet_gateway.shared.id
  ])
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

check "public_subnet_belongs_to_shared_vpc" {
  assert {
    condition     = data.aws_subnet.public.vpc_id == data.aws_vpc.shared.id
    error_message = "public_subnet_id must belong to vpc_id."
  }
}

check "public_subnet_routes_to_shared_internet_gateway" {
  assert {
    condition     = local.public_subnet_has_igw_default_route
    error_message = "The selected public subnet route table must contain 0.0.0.0/0 to the Internet Gateway attached to vpc_id."
  }
}

check "codebuild_subnet_is_private" {
  assert {
    condition = (
      local.codebuild_private_subnet_id != data.aws_subnet.public.id &&
      !local.codebuild_subnet_has_igw_default_route
    )
    error_message = "The selected CodeBuild subnet must differ from the public subnet and must not route directly to the Internet Gateway."
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
