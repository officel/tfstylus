module "example" {
  depends_on = [terraform_data.example]
  ignore_nested_deprecations = true
  providers = {
    aws.dst = aws.west
    aws.src = aws.east
  }
  second = var.second
  version = "~> 1.0"
  source = "example/network/aws"
  first = var.first
  for_each = local.items
  count = 1
}
