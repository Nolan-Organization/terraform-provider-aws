package aws

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/acmpca"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccAwsAcmpcaCertificate_RootCertificate(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_acmpca_certificate.test"
	certificateAuthorityResourceName := "aws_acmpca_certificate_authority.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAwsAcmpcaCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsAcmpcaCertificateConfig_RootCertificate(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAwsAcmpcaCertificateExists(resourceName),
					testAccMatchResourceAttrRegionalARN(resourceName, "arn", "acm-pca", regexp.MustCompile(`certificate-authority/.+/certificate/.+$`)),
					resource.TestCheckResourceAttrSet(resourceName, "certificate"),
					resource.TestCheckResourceAttr(resourceName, "certificate_chain", ""),
					resource.TestCheckResourceAttrPair(resourceName, "certificate_authority_arn", certificateAuthorityResourceName, "arn"),
					resource.TestCheckResourceAttrSet(resourceName, "certificate_signing_request"),
					resource.TestCheckResourceAttr(resourceName, "validity_length", "1"),
					resource.TestCheckResourceAttr(resourceName, "validity_unit", "YEARS"),
					resource.TestCheckResourceAttr(resourceName, "signing_algorithm", "SHA512WITHRSA"),
					testAccCheckResourceAttrGlobalARNNoAccount(resourceName, "template_arn", "acm-pca", "template/RootCACertificate/V1"),
				),
			},
		},
	})
}

func TestAccAwsAcmpcaCertificate_SubordinateCertificate(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_acmpca_certificate.test"
	rootCertificateAuthorityResourceName := "aws_acmpca_certificate_authority.root"
	subordinateCertificateAuthorityResourceName := "aws_acmpca_certificate_authority.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAwsAcmpcaCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsAcmpcaCertificateConfig_SubordinateCertificate(rName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAwsAcmpcaCertificateExists(resourceName),
					testAccMatchResourceAttrRegionalARN(resourceName, "arn", "acm-pca", regexp.MustCompile(`certificate-authority/.+/certificate/.+$`)),
					resource.TestCheckResourceAttrSet(resourceName, "certificate"),
					resource.TestCheckResourceAttrSet(resourceName, "certificate_chain"),
					resource.TestCheckResourceAttrPair(resourceName, "certificate_authority_arn", rootCertificateAuthorityResourceName, "arn"),
					resource.TestCheckResourceAttrPair(resourceName, "certificate_signing_request", subordinateCertificateAuthorityResourceName, "certificate_signing_request"),
					resource.TestCheckResourceAttr(resourceName, "validity_length", "1"),
					resource.TestCheckResourceAttr(resourceName, "validity_unit", "YEARS"),
					resource.TestCheckResourceAttr(resourceName, "signing_algorithm", "SHA512WITHRSA"),
					testAccCheckResourceAttrGlobalARNNoAccount(resourceName, "template_arn", "acm-pca", "template/SubordinateCACertificate_PathLen0/V1"),
				),
			},
		},
	})
}

func TestAccAwsAcmpcaCertificate_EndEntityCertificate(t *testing.T) {
	rName := acctest.RandomWithPrefix("tf-acc-test")
	resourceName := "aws_acmpca_certificate.test"
	csr, _ := tlsRsaX509CertificateRequestPem(4096, "terraformtest1.com")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccCheckAwsAcmpcaCertificateDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsAcmpcaCertificateConfig_EndEntityCertificate(rName, tlsPemEscapeNewlines(csr)),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAwsAcmpcaCertificateExists(resourceName),
					testAccMatchResourceAttrRegionalARN(resourceName, "arn", "acm-pca", regexp.MustCompile(`certificate-authority/.+/certificate/.+$`)),
					resource.TestCheckResourceAttrSet(resourceName, "certificate"),
					resource.TestCheckResourceAttrSet(resourceName, "certificate_chain"),
					resource.TestCheckResourceAttr(resourceName, "certificate_signing_request", csr),
					resource.TestCheckResourceAttr(resourceName, "validity_length", "1"),
					resource.TestCheckResourceAttr(resourceName, "validity_unit", "DAYS"),
					resource.TestCheckResourceAttr(resourceName, "signing_algorithm", "SHA256WITHRSA"),
					testAccCheckResourceAttrGlobalARNNoAccount(resourceName, "template_arn", "acm-pca", "template/EndEntityCertificate/V1"),
				),
			},
		},
	})
}

func testAccCheckAwsAcmpcaCertificateDestroy(s *terraform.State) error {
	conn := testAccProvider.Meta().(*AWSClient).acmpcaconn

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "aws_acmpca_certificate" {
			continue
		}

		input := &acmpca.GetCertificateInput{
			CertificateArn:          aws.String(rs.Primary.ID),
			CertificateAuthorityArn: aws.String(rs.Primary.Attributes["certificate_authority_arn"]),
		}

		output, err := conn.GetCertificate(input)
		if tfawserr.ErrCodeEquals(err, acmpca.ErrCodeResourceNotFoundException) {
			return nil
		}
		if tfawserr.ErrMessageContains(err, acmpca.ErrCodeInvalidStateException, "not in the correct state to have issued certificates") {
			// This is returned when checking root certificates and the certificate has not been associated with the certificate authority
			return nil
		}
		if err != nil {
			return err
		}

		if output != nil {
			return fmt.Errorf("ACM PCA Certificate (%s) still exists", rs.Primary.ID)
		}
	}

	return nil
}

func testAccCheckAwsAcmpcaCertificateExists(resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		conn := testAccProvider.Meta().(*AWSClient).acmpcaconn
		input := &acmpca.GetCertificateInput{
			CertificateArn:          aws.String(rs.Primary.ID),
			CertificateAuthorityArn: aws.String(rs.Primary.Attributes["certificate_authority_arn"]),
		}

		output, err := conn.GetCertificate(input)

		if err != nil {
			return err
		}

		if output == nil || output.Certificate == nil {
			return fmt.Errorf("ACM PCA Certificate %q does not exist", rs.Primary.ID)
		}

		return nil
	}
}

func testAccAwsAcmpcaCertificateConfig_RootCertificate(rName string) string {
	return fmt.Sprintf(`
resource "aws_acmpca_certificate" "test" {
  certificate_authority_arn   = aws_acmpca_certificate_authority.test.arn
  certificate_signing_request = aws_acmpca_certificate_authority.test.certificate_signing_request
  signing_algorithm           = "SHA512WITHRSA"

  template_arn = "arn:${data.aws_partition.current.partition}:acm-pca:::template/RootCACertificate/V1"

  validity_length = 1
  validity_unit   = "YEARS"
}

resource "aws_acmpca_certificate_authority" "test" {
  permanent_deletion_time_in_days = 7
  type                            = "ROOT"

  certificate_authority_configuration {
    key_algorithm     = "RSA_4096"
    signing_algorithm = "SHA512WITHRSA"

    subject {
      common_name = "%[1]s.com"
    }
  }
}

data "aws_partition" "current" {}
`, rName)
}

func testAccAwsAcmpcaCertificateConfig_SubordinateCertificate(rName string) string {
	return fmt.Sprintf(`
resource "aws_acmpca_certificate" "test" {
  certificate_authority_arn   = aws_acmpca_certificate_authority.root.arn
  certificate_signing_request = aws_acmpca_certificate_authority.test.certificate_signing_request
  signing_algorithm           = "SHA512WITHRSA"

  template_arn = "arn:${data.aws_partition.current.partition}:acm-pca:::template/SubordinateCACertificate_PathLen0/V1"

  validity_length = 1
  validity_unit   = "YEARS"
}

resource "aws_acmpca_certificate_authority" "test" {
  permanent_deletion_time_in_days = 7
  type                            = "SUBORDINATE"

  certificate_authority_configuration {
    key_algorithm     = "RSA_2048"
    signing_algorithm = "SHA512WITHRSA"

    subject {
      common_name = "sub.%[1]s.com"
    }
  }
}

resource "aws_acmpca_certificate_authority" "root" {
  permanent_deletion_time_in_days = 7
  type                            = "ROOT"

  certificate_authority_configuration {
    key_algorithm     = "RSA_4096"
    signing_algorithm = "SHA512WITHRSA"

    subject {
      common_name = "%[1]s.com"
    }
  }
}

resource "aws_acmpca_certificate_authority_certificate" "root" {
  certificate_authority_arn = aws_acmpca_certificate_authority.root.arn

  certificate       = aws_acmpca_certificate.root.certificate
  certificate_chain = aws_acmpca_certificate.root.certificate_chain
}

resource "aws_acmpca_certificate" "root" {
  certificate_authority_arn   = aws_acmpca_certificate_authority.root.arn
  certificate_signing_request = aws_acmpca_certificate_authority.root.certificate_signing_request
  signing_algorithm           = "SHA512WITHRSA"

  template_arn = "arn:${data.aws_partition.current.partition}:acm-pca:::template/RootCACertificate/V1"

  validity_length = 2
  validity_unit   = "YEARS"
}

data "aws_partition" "current" {}
`, rName)
}

func testAccAwsAcmpcaCertificateConfig_EndEntityCertificate(rName, csr string) string {
	return fmt.Sprintf(`
resource "aws_acmpca_certificate" "test" {
  certificate_authority_arn   = aws_acmpca_certificate_authority.root.arn
  certificate_signing_request = "%[2]s"
  signing_algorithm           = "SHA256WITHRSA"

  template_arn = "arn:${data.aws_partition.current.partition}:acm-pca:::template/EndEntityCertificate/V1"

  validity_length             = 1
  validity_unit               = "DAYS"
}

resource "aws_acmpca_certificate_authority" "root" {
	permanent_deletion_time_in_days = 7
	type                            = "ROOT"
  
	certificate_authority_configuration {
	  key_algorithm     = "RSA_4096"
	  signing_algorithm = "SHA512WITHRSA"
  
	  subject {
		common_name = "%[1]s.com"
	  }
	}
  }
  
  resource "aws_acmpca_certificate_authority_certificate" "root" {
	certificate_authority_arn = aws_acmpca_certificate_authority.root.arn
  
	certificate       = aws_acmpca_certificate.root.certificate
	certificate_chain = aws_acmpca_certificate.root.certificate_chain
  }
  
  resource "aws_acmpca_certificate" "root" {
	certificate_authority_arn   = aws_acmpca_certificate_authority.root.arn
	certificate_signing_request = aws_acmpca_certificate_authority.root.certificate_signing_request
	signing_algorithm           = "SHA512WITHRSA"
  
	template_arn = "arn:${data.aws_partition.current.partition}:acm-pca:::template/RootCACertificate/V1"
  
	validity_length = 2
	validity_unit   = "YEARS"
  }
  
  data "aws_partition" "current" {}
  `, rName, csr)
}

func TestValidateAcmPcaTemplateArn(t *testing.T) {
	validNames := []string{
		"arn:aws:acm-pca:::template/EndEntityCertificate/V1",                     // lintignore:AWSAT005
		"arn:aws:acm-pca:::template/SubordinateCACertificate_PathLen0/V1",        // lintignore:AWSAT005
		"arn:aws-us-gov:acm-pca:::template/EndEntityCertificate/V1",              // lintignore:AWSAT005
		"arn:aws-us-gov:acm-pca:::template/SubordinateCACertificate_PathLen0/V1", // lintignore:AWSAT005
	}
	for _, v := range validNames {
		_, errors := validateAcmPcaTemplateArn(v, "template_arn")
		if len(errors) != 0 {
			t.Fatalf("%q should be a valid ACM PCA ARN: %q", v, errors)
		}
	}

	invalidNames := []string{
		"arn",
		"arn:aws:s3:::my_corporate_bucket/exampleobject.png",                       // lintignore:AWSAT005
		"arn:aws:acm-pca:us-west-2::template/SubordinateCACertificate_PathLen0/V1", // lintignore:AWSAT003,AWSAT005
		"arn:aws:acm-pca::123456789012:template/EndEntityCertificate/V1",           // lintignore:AWSAT005
		"arn:aws:acm-pca:::not-a-template/SubordinateCACertificate_PathLen0/V1",    // lintignore:AWSAT005
	}
	for _, v := range invalidNames {
		_, errors := validateAcmPcaTemplateArn(v, "template_arn")
		if len(errors) == 0 {
			t.Fatalf("%q should be an invalid ARN", v)
		}
	}
}
