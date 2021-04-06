
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

  ingress {
    from_port   = 443
    to_port     = 443
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

  depends_on = [
    aws_alb.application_load_balancer
  ]

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_lb_listener" "listener" {
  load_balancer_arn = "${aws_alb.application_load_balancer.arn}"
  certificate_arn   = aws_acm_certificate.this.arn
  ssl_policy        = "ELBSecurityPolicy-2016-08"
  port              = "443"
  protocol          = "HTTPS"

  default_action {
    type             = "forward"
    target_group_arn = "${aws_lb_target_group.target_group.arn}"
  }
}

resource "aws_lb_listener" "http_listener" {
  load_balancer_arn = "${aws_alb.application_load_balancer.arn}"
  port              = "80"
  protocol          = "HTTP"

  default_action {
    type             = "redirect"

    redirect {
      port = "443"
      protocol = "HTTPS"
      status_code = "HTTP_301"
    }
  }
}

data "aws_route53_zone" "this" {
  name = var.route53_zone
  private_zone = false
}

resource "aws_route53_record" "this" {
  name = var.dns_record_name
  type = "CNAME"

  records = [
    aws_alb.application_load_balancer.dns_name,
  ]

  zone_id = data.aws_route53_zone.this.zone_id
  ttl = "60"
}

resource "aws_acm_certificate" "this" {
  domain_name       = "${var.dns_record_name}.${var.route53_zone}"
  validation_method = "DNS"

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_acm_certificate_validation" "this" {
  certificate_arn         = aws_acm_certificate.this.arn
  validation_record_fqdns = [aws_route53_record.web_cert_validation.fqdn]

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_route53_record" "web_cert_validation" {
  name = element(tolist(aws_acm_certificate.this.domain_validation_options), 0).resource_record_name
  type = element(tolist(aws_acm_certificate.this.domain_validation_options), 0).resource_record_type

  records = [ 
    element(tolist(aws_acm_certificate.this.domain_validation_options), 0).resource_record_value 
  ]

  zone_id = data.aws_route53_zone.this.zone_id
  ttl     = 60

  lifecycle {
    create_before_destroy = true
  }
}
