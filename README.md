msk
===

assume-role helper.

## installation

```
brew install reiki4040/tap/msk
```

## usage

assume role to `<profile>` and show temporary credentials
```
msk <profile>
```

```
export AWS_ACCESS_KEY_ID="<temporary credential>"
export AWS_SECRET_ACCESS_KEY="<temporary credential>"
export AWS_SESSION_TOKEN="<temporary credential>"
export AWS_SECURITY_TOKEN="<temporary credential>"
export ASSUMED_ROLE="<assumed role arn>"
export AWS_PROFILE="<target profile>"
# this temporary credentials expire at YYYY-MM-DDTHH:mm:ss
```

set credentials to current shell (ex. bash, zsh)
```
eval $(msk <profile>)
```

### -n, --no-export-aws-profile

Suppress `AWS_PROFILE` from the output. Use this when exporting `AWS_PROFILE` causes issues, such as running Claude Code with Bedrock.

```
eval $(msk -n <profile>)
```

### example

`~/.aws/config`
```
[default]
region = ap-northeast-1

[profile dev]
role_arn = arn:aws:iam::<account id>:role/<dev role name>
source_arn = default

[profile admin]
role_arn = arn:aws:iam::<account id>:role/<admin role name>
source_arn = default
mfa_serial = arn:aws:iam::<account id>:mfa/<user name>
```

assume role to `dev` profile from `default` profile.
```
msk dev
```

assume role to `admin` profile from `default` profile with MFA (auto detect from profile setting)
```
msk other
MFA code: 
```

set temporary `dev` credentials to current shell 
```
eval $(msk dev)
```

assume role check with `aws cli` after `eval`
```
aws sts get-caller-identity
```

```
{
    "UserId": "<ID>:via-msk",
    "Account": "<account id>",
    "Arn": "arn:aws:sts::<account id>:assumed-role/<assumed role>/via-msk"
}
```