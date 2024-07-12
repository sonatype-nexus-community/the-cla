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

variable "aws_region" {
  description = "AWS Region that our deployment is targetting"
  type        = string
  default     = "eu-west-2"
}

variable "default_resource_tags" {
  description = "List of tags to apply to all resources created in AWS"
  type        = map(string)
  default = {
    environment : "production"
    purpose : "sonatype-community-cla"
    owner : "phorton@sonatype.com"
    sonatype-group : "se"
  }
}

# See https://docs.sonatype.com/display/OPS/Shared+Infrastructure+Initiative
variable "environment" {
  description = "Used as part of Sonatype's Shared AWS Infrastructure"
  type        = string
  default     = "production"
}

variable "the_cla_pem" {
  description = "See the-cla.pem"
  type = string
  sensitive = true
}

variable "env_cla_url" {
  description = "See CLA_URL"
  type = string
  default = "https://s3.amazonaws.com/sonatype-cla/cla.txt"
}

variable "env_gh_app_id" {
  description = "See GH_APP_ID"
  type = string
  sensitive = true
}

variable "env_github_client_secret" {
  description = "See GITHUB_CLIENT_SECRET"
  type = string
  sensitive = true
}

variable "env_github_webhook_secret" {
  description = "See GH_WEBHOOK_SECRET"
  type = string
  sensitive = true
}

variable "env_react_app_company_name" {
  description = "See REACT_APP_COMPANY_NAME"
  type = string
  default = "Sonatype Open Source Community"
}

variable "env_react_app_company_website" {
  description = "See REACT_APP_COMPANY_WEBSITE"
  type = string
  default = "https://www.sonatype.com/"
}

variable "env_react_app_cla_app_name" {
  description = "See REACT_APP_CLA_APP_NAME"
  type = string
  default = "Sonatype Open Source Community CLA"
}

variable "env_react_app_cla_version" {
  description = "See REACT_APP_CLA_VERSION"
  type = string
}

variable "env_react_app_gh_client_id" {
  description = "See REACT_APP_GITHUB_CLIENT_ID"
  type = string
  sensitive = true
}