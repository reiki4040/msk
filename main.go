package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

var (
	version  string
	revision string

	optUsage                bool
	optSourceProfile        string
	optDuration             time.Duration
	optFormatEnv            bool
	optNoExportAWSProfile   bool
)

func init() {
	flag.StringVar(&optSourceProfile, "source-profle", "default", "switch source profile")
	flag.DurationVar(&optDuration, "d", time.Hour, "duration seconds for session expire")
	flag.BoolVar(&optUsage, "h", false, "show usage.")
	flag.BoolVar(&optUsage, "help", false, "show usage.")
	flag.BoolVar(&optFormatEnv, "format-env", false, "output format `K=V`")
	flag.BoolVar(&optNoExportAWSProfile, "n", false, "no export AWS_PROFILE")
	flag.BoolVar(&optNoExportAWSProfile, "no-export-aws-profile", false, "no export AWS_PROFILE")
	flag.Parse()
}

func showHelp() {
	usage := `
msk is assume role helper.
you can set temporary assume role credentials to current zsh/bash.

  eval $(msk <profile>)

  * profile in ~/.aws/config

[Usage]

  assume role to profile and show credential export.

  # for eval, zshrc or bashrc
  msk <profile>

    export AWS_ACCESS_KEY_ID="<temporary credential>"
    export AWS_SECRET_ACCESS_KEY="<temporary credential>"
    export AWS_SESSION_TOKEN="<temporary credential>"
    export AWS_SECURITY_TOKEN="<temporary credential>"
    export ASSUMED_ROLE="<assumed role arn>"
    export AWS_PROFILE="<target profile>"
    # this temporary credentials expire at YYYY-MM-DDTHH:mm:ss

  # no export AWS_PROFILE
  msk -n <profile>

  # for .env file
  msk -format-env <profile>

    AWS_ACCESS_KEY_ID="<temporary credential>"
    AWS_SECRET_ACCESS_KEY="<temporary credential>"
    AWS_SESSION_TOKEN="<temporary credential>"
    AWS_SECURITY_TOKEN="<temporary credential>"
    ASSUMED_ROLE="<assumed role arn>"
    AWS_PROFILE="<target profile>"
    # this temporary credentials expire at YYYY-MM-DDTHH:mm:ss

[Optoins]
`
	usageLast := `
see example:
https://github.com/reiki4040/msk?tab=readme-ov-file#example
`
	fmt.Printf("msk %s[%s]\n", version, revision)
	fmt.Println(usage)
	flag.PrintDefaults()
	fmt.Println(usageLast)
}

func main() {
	if optUsage {
		showHelp()
		return
	}

	if len(flag.Args()) != 1 {
		log.Fatal("required profile")
	}
	targetProfile := flag.Args()[0]

	ctx := context.Background()
	// load target profile from ~/.aws/config
	cnf, err := config.LoadSharedConfigProfile(ctx, targetProfile)
	if err != nil {
		log.Fatal(err)
	}

	// check role arn
	roleArn, err := arn.Parse(cnf.RoleARN)
	if err != nil {
		log.Fatal(err)
	}
	if roleArn.Service != "iam" && roleArn.Resource != "role" {
		log.Fatal("invalid role_arn in config")
	}
	role := cnf.RoleARN

	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithSharedConfigProfile(optSourceProfile), // always set source profile, because current shell exported temporary credentials.
	)
	if err != nil {
		log.Fatal(err)
	}
	stsCli := sts.NewFromConfig(cfg)

	sessionName := "via-msk"
	expireIn := int32(optDuration / time.Second)
	in := &sts.AssumeRoleInput{
		RoleArn:         aws.String(role),
		RoleSessionName: aws.String(sessionName),
		DurationSeconds: aws.Int32(expireIn),
	}

	if cnf.MFASerial != "" {
		// check mfa arn
		mfaArn, err := arn.Parse(cnf.MFASerial)
		if err != nil {
			log.Fatal(err)
		}
		if mfaArn.Service != "iam" && mfaArn.Resource != "mfa" {
			log.Fatal("invalid role_arn in config")
		}

		// read MFA token from terminal
		mfaNum, err := readTokenCode()
		if err != nil {
			log.Fatal(err)
		}
		in.SerialNumber = aws.String(cnf.MFASerial)
		in.TokenCode = aws.String(mfaNum)
	}

	resp, err := stsCli.AssumeRole(ctx, in)
	if err != nil {
		log.Fatal(err)
	}

	AwsKey := *resp.Credentials.AccessKeyId
	AwsSecret := *resp.Credentials.SecretAccessKey
	AwsSessionToken := *resp.Credentials.SessionToken
	assumedRole := *resp.AssumedRoleUser.Arn
	expire := resp.Credentials.Expiration.Format(time.RFC3339)

	if optFormatEnv {
		fmt.Printf("AWS_ACCESS_KEY_ID=\"%s\"\n", AwsKey)
		fmt.Printf("AWS_SECRET_ACCESS_KEY=\"%s\"\n", AwsSecret)
		fmt.Printf("AWS_SESSION_TOKEN=\"%s\"\n", AwsSessionToken)
		fmt.Printf("AWS_SECURITY_TOKEN=\"%s\"\n", AwsSessionToken)
		fmt.Printf("ASSUMED_ROLE=\"%s\"\n", assumedRole)
		if !optNoExportAWSProfile {
			fmt.Printf("AWS_PROFILE=\"%s\"\n", targetProfile)
		}
		fmt.Printf("# this temporary credentials expire at %s\n", expire)
	} else {
		fmt.Printf("export AWS_ACCESS_KEY_ID=\"%s\"\n", AwsKey)
		fmt.Printf("export AWS_SECRET_ACCESS_KEY=\"%s\"\n", AwsSecret)
		fmt.Printf("export AWS_SESSION_TOKEN=\"%s\"\n", AwsSessionToken)
		fmt.Printf("export AWS_SECURITY_TOKEN=\"%s\"\n", AwsSessionToken)
		fmt.Printf("export ASSUMED_ROLE=\"%s\"\n", assumedRole)
		if !optNoExportAWSProfile {
			fmt.Printf("export AWS_PROFILE=\"%s\"\n", targetProfile)
		}
		fmt.Printf("# this temporary credentials expire at %s\n", expire)
	}
}

/*
Why does NOT use stscreds.StdinTokenProiver()?
https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/credentials/stscreds#StdinTokenProvider
stscreds.StdinTokenProiver() shows prompt to Stdout.
use stderr because assuming that the result will be eval().
*/
func readTokenCode() (string, error) {
	fmt.Fprintf(os.Stderr, "MFA code: ")
	mfaCodeBytes, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(mfaCodeBytes)), nil
}
