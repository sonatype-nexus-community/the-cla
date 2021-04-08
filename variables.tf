
variable "availability_zone_names" {
  type = list(string)
  default = ["us-east-1"]
}

variable "app_name" {
  type = string
  default = "the-cla"
}

variable "aws_region" {
  type = string
  default = "us-east-1"
}

variable "route53_zone" {
  type = string
  default = "example.host.com"
}

variable "dns_record_name" {
  type = string
  default = "the-cla"
}

variable "postgres_username" {
  type = string
  default = "the_cla"
  sensitive = true
}

variable "postgres_password" {
  type = string
  sensitive = true
}

variable "postgres_db_name" {
  type = string 
  default = "thecladatabase"
  sensitive = true
}

variable "external_db_cidr_group" {
  type = string
  sensitive = true
}
