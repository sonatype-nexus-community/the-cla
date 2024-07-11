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

################################################################################
# Load Vendor Corp Shared Infra
################################################################################
module "shared" {
  source                   = "git::ssh://git@github.com/vendorcorp/terraform-shared-infrastructure.git?ref=v1.2.0"
}

module "shared_private" {
  source                   = "git::ssh://git@github.com/vendorcorp/terraform-shared-private-infrastructure.git?ref=v1.5.0"
  environment              = var.environment
}

module "database" {
  source            = "git::ssh://git@github.com/vendorcorp/terraform-aws-rds-database.git?ref=v0.1.0"

  pg_hostname       = module.shared.pgsql_cluster_endpoint_write
  pg_port           = module.shared.pgsql_cluster_port
  pg_admin_username = module.shared.pgsql_cluster_master_username
  pg_admin_password = module.shared.pgsql_cluster_master_password
  database_name     = local.cla_db_name
  user_username     = local.cla_db_username
  # Password generated and returned
}