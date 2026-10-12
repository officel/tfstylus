# header
locals { keep = "same" }

list "aws_instance" "example" {
  for_each = { west = "eu" } # each
  count = 1
  provider = aws.west
  include_resource = false
  # result cap
  limit = 100
  custom_filter = var.extra
  config {
    filter = {
      name = "web"
    }
    # provider-specific comment
    limit = 20
  }
}

# footer
