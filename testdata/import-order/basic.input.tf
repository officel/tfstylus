import {
  id = "bucket-123"
  # destination resource
  to       = aws_s3_bucket.this
  provider = aws.west
  for_each = local.buckets
}
