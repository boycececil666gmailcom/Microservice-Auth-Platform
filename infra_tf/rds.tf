#region RDS Password
resource "random_password" "db_password" {
  length  = 20
  special = false
}
#endregion

#region RDS Subnet Group
resource "aws_db_subnet_group" "postgres" {
  name        = "${var.app_name}-db-subnet-group-${var.environment}"
  description = "Subnet group for PostgreSQL RDS"
  subnet_ids  = aws_subnet.private[*].id
}
#endregion

#region RDS PostgreSQL
resource "aws_db_instance" "postgres" {
  identifier                  = "${var.app_name}-db-${var.environment}"
  engine                      = "postgres"
  engine_version              = "16.9"
  instance_class              = "db.t4g.micro"
  allocated_storage           = 20
  max_allocated_storage       = 20
  storage_type                = "gp3"
  db_name                     = "urlshortener"
  username                    = "postgres"
  password                    = random_password.db_password.result
  db_subnet_group_name        = aws_db_subnet_group.postgres.name
  vpc_security_group_ids      = [aws_security_group.rds.id]
  backup_retention_period     = 0
  skip_final_snapshot         = true
  deletion_protection         = false
  performance_insights_enabled = false
  monitoring_interval         = 0

  tags = {
    Name = "${var.app_name}-postgres-${var.environment}"
  }
}
#endregion
