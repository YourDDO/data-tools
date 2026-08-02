resource "aws_eip" "nat" {
  domain = "vpc"

  tags = {
    Name = "${var.project_name}-nat"
  }
}

resource "aws_nat_gateway" "codebuild" {
  allocation_id = aws_eip.nat.id
  subnet_id     = data.aws_subnet.public.id

  tags = {
    Name = "${var.project_name}-codebuild"
  }

  lifecycle {
    precondition {
      condition     = local.public_subnet_has_igw_default_route
      error_message = "The NAT Gateway public subnet must route 0.0.0.0/0 to the VPC Internet Gateway."
    }
  }
}

resource "aws_route" "codebuild_default_ipv4" {
  route_table_id         = data.aws_route_table.codebuild_private.id
  destination_cidr_block = "0.0.0.0/0"
  nat_gateway_id         = aws_nat_gateway.codebuild.id

  lifecycle {
    precondition {
      condition = (
        local.codebuild_private_subnet_id != data.aws_subnet.public.id &&
        !local.codebuild_subnet_has_igw_default_route
      )
      error_message = "CodeBuild must use a private subnet without a direct Internet Gateway default route."
    }
  }
}

resource "aws_vpc_security_group_ingress_rule" "codebuild_to_compendium" {
  security_group_id            = data.aws_security_group.compendium.id
  referenced_security_group_id = module.codebuild.codebuild_security_group_id

  description = "Allow YourDDO data pipeline to reach the Compendium API"

  ip_protocol = "tcp"
  from_port   = 80
  to_port     = 80
}
