module "example" {
  source = "example/network/aws"
  version = "~> 1.0"
  for_each = local.items
  count = 1
  second = var.second
  first = var.first
  providers = {
    aws.dst = aws.west
    aws.src = aws.east
  }
  ignore_nested_deprecations = true
  depends_on = [terraform_data.example]
}
