package tests

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func Test_IntegrationSuite(t *testing.T) {
	suite.Run(t, newIntegrationSuite())
}
