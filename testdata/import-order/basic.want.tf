import {
  for_each = local.buckets
  provider = aws.west
  id       = "bucket-123"
  # destination resource
  to = aws_s3_bucket.this
}
