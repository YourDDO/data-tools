resource "aws_vpc_security_group_ingress_rule" "codebuild_to_compendium" {
  security_group_id            = data.aws_security_group.compendium.id
  referenced_security_group_id = module.codebuild.codebuild_security_group_id

  description = "Allow YourDDO data pipeline to reach the Compendium API"

  ip_protocol = "tcp"
  from_port   = 80
  to_port     = 80
}
