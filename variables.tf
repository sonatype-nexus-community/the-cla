
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
