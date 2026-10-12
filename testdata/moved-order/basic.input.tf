locals {
  keep = "unchanged"
}

moved {
  # destination
  to = module.new.aws_instance.this["key"] # keep destination
  # source
  from = module.old.aws_instance.this["key"]
}

moved {
  from = aws_instance.other
  to   = aws_instance.another
}
