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

resource "random_string" "info_username_suffix" {
  length  = 10
  special = false
}

resource "random_string" "info_user_password" {
  length  = 20
  special = false
}

locals {
  cla_db_username  = "the_cla_bot"
  cla_db_name = "the_cla"
  info_username = "info-user-${random_string.info_username_suffix.result}"
  info_password = "${random_string.info_user_password.result}"
}