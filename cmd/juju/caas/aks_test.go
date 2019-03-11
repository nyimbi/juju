// Copyright 2019 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caas

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/mock/gomock"
	"github.com/juju/cmd"
	"github.com/juju/cmd/cmdtesting"
	"github.com/juju/juju/cmd/juju/caas/mocks"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	"github.com/juju/utils/exec"
	gc "gopkg.in/check.v1"
)

type aksSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&aksSuite{})

func (s *aksSuite) SetUpTest(c *gc.C) {
	s.IsolationSuite.SetUpTest(c)
	err := os.Setenv("PATH", "/path/to/here")
	c.Assert(err, jc.ErrorIsNil)
}

func (s *aksSuite) TestGetKubeConfig(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	mockRunner := mocks.NewMockCommandRunner(ctrl)
	configFile := filepath.Join(c.MkDir(), "config")
	err := os.Setenv("KUBECONFIG", configFile)
	c.Assert(err, jc.ErrorIsNil)
	aks := &aks{
		CommandRunner: mockRunner,
		azExecName:    "az",
	}
	err = ioutil.WriteFile(configFile, []byte("data"), 0644)
	c.Assert(err, jc.ErrorIsNil)

	gomock.InOrder(
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "az aks get-credentials --name mycluster --resource-group resourceGroup --overwrite-existing -f " + configFile,
			Environment: []string{"KUBECONFIG=" + configFile, "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code: 0,
			}, nil),
	)

	rdr, clusterName, err := aks.getKubeConfig(&clusterParams{
		name:          "mycluster",
		resourceGroup: "resourceGroup",
	})
	c.Check(err, jc.ErrorIsNil)
	defer rdr.Close()

	c.Assert(clusterName, gc.Equals, "mycluster")
	data, err := ioutil.ReadAll(rdr)
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(string(data), gc.DeepEquals, "data")
}

func (s *aksSuite) TestInteractiveParam(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	mockRunner := mocks.NewMockCommandRunner(ctrl)
	aks := &aks{
		CommandRunner: mockRunner,
		azExecName:    "az",
	}
	resourceGroup := "testRG"
	clusterName := "mycluster"
	clusterJSONResp := fmt.Sprintf(`
[
  {
    "name": "%s",
    "resourceGroup": "%s"
  },
  {
    "name": "notThisCluster",
    "resourceGroup": "notThisRG"
  }
]
`, clusterName, resourceGroup)

	gomock.InOrder(
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "az aks list --output json",
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code:   0,
				Stdout: []byte(clusterJSONResp),
			}, nil),
	)
	stdin := strings.NewReader("mycluster in resource group testRG\n")
	out := &bytes.Buffer{}
	ctx := &cmd.Context{
		Dir:    c.MkDir(),
		Stdout: out,
		Stderr: ioutil.Discard,
		Stdin:  stdin,
	}
	expected := `
Available Clusters
  mycluster in resource group testRG
  notThisCluster in resource group notThisRG

Select cluster [mycluster in resource group testRG]: 
`[1:]

	outParams, err := aks.interactiveParams(ctx, &clusterParams{})
	c.Check(err, jc.ErrorIsNil)
	c.Check(cmdtesting.Stdout(ctx), gc.Equals, expected)
	c.Assert(outParams, jc.DeepEquals, &clusterParams{
		name:          clusterName,
		resourceGroup: resourceGroup,
	})
}

func (s *aksSuite) TestInteractiveParamResourceGroupDefined(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	mockRunner := mocks.NewMockCommandRunner(ctrl)
	aks := &aks{
		CommandRunner: mockRunner,
		azExecName:    "az",
	}
	resourceGroup := "testRG"
	clusterName := "mycluster"
	clusterJSONResp := fmt.Sprintf(`
[
  {
    "name": "%s",
    "resourceGroup": "%s"
  }
]
`, clusterName, resourceGroup)
	gomock.InOrder(
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "az aks list --output json --resource-group " + resourceGroup,
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code:   0,
				Stdout: []byte(clusterJSONResp),
			}, nil),
	)
	stdin := strings.NewReader("mycluster\n")
	out := &bytes.Buffer{}
	ctx := &cmd.Context{
		Dir:    c.MkDir(),
		Stdout: out,
		Stderr: ioutil.Discard,
		Stdin:  stdin,
	}
	expected := `
Available Clusters
  mycluster

Select cluster [mycluster]: 
`[1:]

	outParams, err := aks.interactiveParams(ctx, &clusterParams{
		resourceGroup: resourceGroup,
	})
	c.Check(err, jc.ErrorIsNil)
	c.Check(cmdtesting.Stdout(ctx), gc.Equals, expected)
	c.Assert(outParams, jc.DeepEquals, &clusterParams{
		name:          clusterName,
		resourceGroup: resourceGroup,
	})
}

func (s *aksSuite) TestInteractiveParamsNoResourceGroupSpecifiedSingleResourceGroupInUse(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	mockRunner := mocks.NewMockCommandRunner(ctrl)
	aks := &aks{
		CommandRunner: mockRunner,
		azExecName:    "az",
	}
	resourceGroup := "testRG"
	clusterName := "mycluster"
	clusterJSONResp := fmt.Sprintf(`
[
  {
    "name": "%s",
    "resourceGroup": "%s"
  },
  {
    "name": "notMeSir",
    "resourceGroup": "%s"
  }
]
`, clusterName, resourceGroup, resourceGroup)
	gomock.InOrder(
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "az aks list --output json",
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code:   0,
				Stdout: []byte(clusterJSONResp),
			}, nil),
	)
	stdin := strings.NewReader("mycluster\n")
	out := &bytes.Buffer{}
	ctx := &cmd.Context{
		Dir:    c.MkDir(),
		Stdout: out,
		Stderr: ioutil.Discard,
		Stdin:  stdin,
	}
	expected := `
Available Clusters In Resource Group TestRG
  mycluster
  notMeSir

Select cluster [mycluster]: 
`[1:]

	outParams, err := aks.interactiveParams(ctx, &clusterParams{})
	c.Check(err, jc.ErrorIsNil)
	c.Check(cmdtesting.Stdout(ctx), gc.Equals, expected)
	c.Assert(outParams, jc.DeepEquals, &clusterParams{
		name:          clusterName,
		resourceGroup: resourceGroup,
	})
}

func (s *aksSuite) TestInteractiveParamsNoResourceGroupSpecifiedMultiResourceGroupsInUse(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	mockRunner := mocks.NewMockCommandRunner(ctrl)
	aks := &aks{
		CommandRunner: mockRunner,
		azExecName:    "az",
	}
	resourceGroup := "testRG"
	clusterName := "mycluster"
	clusterJSONResp := fmt.Sprintf(`
[
  {
    "name": "%s",
    "resourceGroup": "%s"
  },
  {
    "name": "notMeSir",
    "resourceGroup": "MonsterResourceGroup"
  }
]
`, clusterName, resourceGroup)
	gomock.InOrder(
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "az aks list --output json",
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code:   0,
				Stdout: []byte(clusterJSONResp),
			}, nil),
	)
	stdin := strings.NewReader("mycluster in resource group testRG\n")
	out := &bytes.Buffer{}
	ctx := &cmd.Context{
		Dir:    c.MkDir(),
		Stdout: out,
		Stderr: ioutil.Discard,
		Stdin:  stdin,
	}
	expected := `
Available Clusters
  mycluster in resource group testRG
  notMeSir in resource group MonsterResourceGroup

Select cluster [mycluster in resource group testRG]: 
`[1:]

	outParams, err := aks.interactiveParams(ctx, &clusterParams{})
	c.Check(err, jc.ErrorIsNil)
	c.Check(cmdtesting.Stdout(ctx), gc.Equals, expected)
	c.Assert(outParams, jc.DeepEquals, &clusterParams{
		name:          clusterName,
		resourceGroup: resourceGroup,
	})
}

func (s *aksSuite) TestInteractiveParamsClusterSpecifiedNoResourceGroupSpecifiedSingleGroupInUse(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	mockRunner := mocks.NewMockCommandRunner(ctrl)
	aks := &aks{
		CommandRunner: mockRunner,
		azExecName:    "az",
	}
	resourceGroup := "testRG"
	clusterName := "mycluster"
	clusterJSONResp := fmt.Sprintf(`
[
  {
    "name": "%s",
    "resourceGroup": "%s"
  },
  {
    "name": "notMeCluster",
    "resourceGroup": "%s"
  }
]`, clusterName, resourceGroup, resourceGroup)

	gomock.InOrder(
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "az aks list --output json",
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code:   0,
				Stdout: []byte(clusterJSONResp),
			}, nil),
	)
	stdin := strings.NewReader("")
	out := &bytes.Buffer{}
	ctx := &cmd.Context{
		Dir:    c.MkDir(),
		Stdout: out,
		Stderr: ioutil.Discard,
		Stdin:  stdin,
	}
	expected := ""
	outParams, err := aks.interactiveParams(ctx, &clusterParams{
		name: clusterName,
	})
	c.Check(err, jc.ErrorIsNil)
	c.Check(cmdtesting.Stdout(ctx), gc.Equals, expected)
	c.Assert(outParams, jc.DeepEquals, &clusterParams{
		name:          clusterName,
		resourceGroup: resourceGroup,
	})
}

func (s *aksSuite) TestInteractiveParamsClusterSpecifiedNoResourceGroupSpecifiedMultiClusterInUse(c *gc.C) {
	// If a cluster name is given but there are multiple clusters of that
	// name in different resource groups the user must be prompted to choose
	// which of those resource groups is the correct one.
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	mockRunner := mocks.NewMockCommandRunner(ctrl)
	aks := &aks{
		CommandRunner: mockRunner,
		azExecName:    "az",
	}
	resourceGroup := "testRG"
	clusterName := "mycluster"
	clusterJSONResp := fmt.Sprintf(`
[
  {
    "name": "%s",
    "resourceGroup": "%s"
  },
  {
    "name": "%s",
    "resourceGroup": "notMeGroup"
  },
  {
    "name": "notMeCluster",
    "resourceGroup": "differentRG"
  }
]`, clusterName, resourceGroup, clusterName)

	resourcegroupJSONResp := fmt.Sprintf(`
[
  {
    "location": "westus2",
    "name": "%s"
  },
  {
    "location": "westus2",
    "name": "notMeGroup"
  }
]`, resourceGroup)

	gomock.InOrder(
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "az aks list --output json",
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code:   0,
				Stdout: []byte(clusterJSONResp),
			}, nil),
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    `az group list --output json --query "[?properties.provisioningState=='Succeeded']"`,
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code:   0,
				Stdout: []byte(resourcegroupJSONResp),
			}, nil),
	)
	stdin := strings.NewReader("testRG in westus2")
	out := &bytes.Buffer{}
	ctx := &cmd.Context{
		Dir:    c.MkDir(),
		Stdout: out,
		Stderr: ioutil.Discard,
		Stdin:  stdin,
	}
	expected := `
Available Resource Groups
  testRG in westus2
  notMeGroup in westus2

Select resource group [testRG in westus2]: 
`[1:]
	outParams, err := aks.interactiveParams(ctx, &clusterParams{
		name: clusterName,
	})
	c.Check(err, jc.ErrorIsNil)
	c.Check(cmdtesting.Stdout(ctx), gc.Equals, expected)
	c.Assert(outParams, jc.DeepEquals, &clusterParams{
		name:          clusterName,
		resourceGroup: resourceGroup,
	})
}

func (s *aksSuite) TestInteractiveParamsClusterSpecifiedResourceGroupSpecified(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	mockRunner := mocks.NewMockCommandRunner(ctrl)
	aks := &aks{
		CommandRunner: mockRunner,
		azExecName:    "az",
	}
	resourceGroup := "testRG"
	clusterName := "mycluster"

	stdin := strings.NewReader("")
	out := &bytes.Buffer{}
	ctx := &cmd.Context{
		Dir:    c.MkDir(),
		Stdout: out,
		Stderr: ioutil.Discard,
		Stdin:  stdin,
	}
	expected := ""
	outParams, err := aks.interactiveParams(ctx, &clusterParams{
		name:          clusterName,
		resourceGroup: resourceGroup,
	})
	c.Check(err, jc.ErrorIsNil)
	c.Check(cmdtesting.Stdout(ctx), gc.Equals, expected)
	c.Assert(outParams, jc.DeepEquals, &clusterParams{
		name:          clusterName,
		resourceGroup: resourceGroup,
	})
}

func (s *aksSuite) TestEnsureExecutablePicksAZ(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	mockRunner := mocks.NewMockCommandRunner(ctrl)
	aks := &aks{CommandRunner: mockRunner}

	gomock.InOrder(
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "which az",
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code:   0,
				Stdout: []byte(""),
			}, nil),
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "az account show",
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code:   0,
				Stdout: []byte(""),
			}, nil),
	)
	err := aks.ensureExecutable()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(aks.azExecName, gc.Equals, "az")
}

func (s *aksSuite) TestEnsureExecutableTriesSnap(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	mockRunner := mocks.NewMockCommandRunner(ctrl)
	aks := &aks{CommandRunner: mockRunner}

	gomock.InOrder(
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "which az",
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code: 1,
			}, nil),
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "which azure-cli",
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code: 0,
			}, nil),
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "azure-cli account show",
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code:   0,
				Stdout: []byte(""),
			}, nil),
	)
	err := aks.ensureExecutable()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(aks.azExecName, gc.Equals, "azure-cli")
}

func (s *aksSuite) TestEnsureExecutableRemembersFindings(c *gc.C) {
	ctrl := gomock.NewController(c)
	defer ctrl.Finish()

	mockRunner := mocks.NewMockCommandRunner(ctrl)
	aks := &aks{CommandRunner: mockRunner}

	gomock.InOrder(
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "which az",
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code: 0,
			}, nil),
		mockRunner.EXPECT().RunCommands(exec.RunParams{
			Commands:    "az account show",
			Environment: []string{"KUBECONFIG=", "PATH=/path/to/here"},
		}).Times(1).
			Return(&exec.ExecResponse{
				Code:   0,
				Stdout: []byte(""),
			}, nil),
	)
	err := aks.ensureExecutable()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(aks.azExecName, gc.Equals, "az")
	// 2nd run shouldn't call anything
	err = aks.ensureExecutable()
	c.Assert(err, jc.ErrorIsNil)
	c.Assert(aks.azExecName, gc.Equals, "az")
}
