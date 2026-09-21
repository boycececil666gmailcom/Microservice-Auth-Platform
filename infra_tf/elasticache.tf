#region ElastiCache Subnet
resource "aws_elasticache_subnet_group" "redis" {
  name        = "${var.app_name}-cache-subnet-group-${var.environment}"
  description = "Subnet group for ElastiCache Redis"
  subnet_ids  = aws_subnet.private[*].id
}
#endregion

#region ElastiCache Redis
resource "aws_elasticache_cluster" "redis" {
  cluster_id           = "${var.app_name}-redis-${var.environment}"
  engine               = "redis"
  node_type            = "cache.t4g.micro"
  num_cache_nodes      = 1
  parameter_group_name     = "default.redis7"
  port                     = 6379
  subnet_group_name        = aws_elasticache_subnet_group.redis.name
  security_group_ids       = [aws_security_group.redis.id]
  snapshot_retention_limit = 0

  tags = {
    Name = "${var.app_name}-redis-${var.environment}"
  }
}
#endregion
