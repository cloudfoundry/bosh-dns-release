resource "aws_iam_policy" "bosh_2" {
  name = "${var.env_id}_bosh_policy_2"
  path = "/"

  policy = <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
        "Sid": "RequiredIfUsingAdvertisedRoutesCloudProperties",
        "Effect": "Allow",
        "Action": [
            "ec2:CreateRoute",
            "ec2:DescribeRouteTables",
            "ec2:ReplaceRoute"
        ],
        "Resource": "*"
    }
  ]
}
EOF
}

resource "aws_iam_role_policy_attachment" "bosh_2" {
  role       = "${var.env_id}_bosh_role"
  policy_arn = "${aws_iam_policy.bosh_2.arn}"
}
