
data "aws_vpc" "main" {
    tags = {
        "use-case"    = "shared-vpcs"
        "environment" = "ci"
    }
}
 
data "aws_subnet_ids" "private" {
    vpc_id = data.aws_vpc.main.id
 
    tags = {
        access = "private"
    }
}
 
data "aws_subnet_ids" "public" {
    vpc_id = data.aws_vpc.main.id
 
    tags = {
        access = "public"
    }
}

resource "aws_ecr_repository" "the_cla" {
  name = "${var.app_name}-app"
}

resource "aws_ecs_cluster" "the_cla" {
  name = "${var.app_name}-cluster"
}

resource "aws_ecs_task_definition" "the_cla" {
  family                   = "${var.app_name}-task"
  container_definitions    = <<DEFINITION
  [
    {
      "name": "${var.app_name}-task",
      "image": "${aws_ecr_repository.the_cla.repository_url}:latest",
      "essential": true,
      "portMappings": [
        {
          "containerPort": 4200,
          "hostPort": 4200
        }
      ],
      "memory": 512,
      "cpu": 256
    }
  ]
  DEFINITION
  requires_compatibilities = ["FARGATE"]
  network_mode             = "awsvpc"
  memory                   = 512
  cpu                      = 256
  execution_role_arn       = "${aws_iam_role.ecsTaskExecutionRole.arn}"
}

resource "aws_iam_role" "ecsTaskExecutionRole" {
  name_prefix        = "ecsTaskExecutionRole"
  assume_role_policy = "${data.aws_iam_policy_document.assume_role_policy.json}"
}

data "aws_iam_policy_document" "assume_role_policy" {
  statement {
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role_policy_attachment" "ecsTaskExecutionRole_policy" {
  role       = "${aws_iam_role.ecsTaskExecutionRole.name}"
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_ecs_service" "the_cla" {
  name            = "${var.app_name}-service"
  cluster         = "${aws_ecs_cluster.the_cla.id}"
  task_definition = "${aws_ecs_task_definition.the_cla.arn}"
  launch_type     = "FARGATE"
  desired_count   = 3

  load_balancer {
    target_group_arn = "${aws_lb_target_group.target_group.arn}"
    container_name   = "${aws_ecs_task_definition.the_cla.family}"
    container_port   = 4200
  }

  network_configuration {
    subnets          = data.aws_subnet_ids.public.ids
    assign_public_ip = true
    security_groups  = ["${aws_security_group.service_security_group.id}"]
  }
}

resource "aws_alb" "application_load_balancer" {
  name               = "${var.app_name}-tf"
  load_balancer_type = "application"
  subnets = data.aws_subnet_ids.public.ids
  
  security_groups = ["${aws_security_group.load_balancer_security_group.id}"]
}

resource "aws_security_group" "service_security_group" {
  vpc_id = data.aws_vpc.main.id

  ingress {
    from_port = 0
    to_port   = 0
    protocol  = "-1"
    security_groups = ["${aws_security_group.load_balancer_security_group.id}"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "load_balancer_security_group" {
  vpc_id = data.aws_vpc.main.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_lb_target_group" "target_group" {
  name        = "target-group"
  port        = 80
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = data.aws_vpc.main.id
  health_check {
    matcher = "200,301,302"
    path = "/"
  }
}

resource "aws_lb_listener" "listener" {
  load_balancer_arn = "${aws_alb.application_load_balancer.arn}"
  port              = "80"
  protocol          = "HTTP"
  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.target_group.arn}"
  }
}
