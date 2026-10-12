locals {
  keep = "unchanged"
}

moved {
  # source
  from = module.old.aws_instance.this["key"]
  # destination
  to = module.new.aws_instance.this["key"] # keep destination
}

moved {
  from = aws_instance.other
  to   = aws_instance.another
}
