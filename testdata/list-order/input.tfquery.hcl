# header
locals { keep = "same" }

list "aws_instance" "example" {
  config {
    filter = {
      name = "web"
    }
    # provider-specific comment
    limit = 20
  }
  # result cap
  limit = 100
  include_resource = false
  provider = aws.west
  for_each = { west = "eu" } # each
  count = 1
  custom_filter = var.extra
}

# footer
