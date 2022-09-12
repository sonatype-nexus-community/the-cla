#
# Copyright (c) 2021-present Sonatype, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#


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
