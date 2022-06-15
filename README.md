<h1 align="center">Trickest Client<a href="#"> <img src="https://img.shields.io/badge/Tweet--lightgrey?logo=twitter&style=social" alt="Tweet" height="20"/></a></h1>

<h3 align="center">
Client used for executing, listing, downloading, getting, creating, deleting and searching objects on the <a href=https://trickest.com>Trickest</a> platform.
</h3>
<br>

![Trickest Client](trickest-cli.png "Trickest Client")


# About

Trickest platform is an IDE tailored for bug bounty hunters, penetration testers, and SecOps teams to build and automate workflows from start to finish. Powered by the world's most advanced crowdsourced intelligence.

Current workflow categories:

- Recon
- Discovery
- Fuzzing
- Network
- Containers
- Scanners
- Social Engineering
- Cloud Storage
- Static Code Analysis
- Utilities

# Installation

#### **macOS**

```
# Download the binary
curl -sLO https://github.com/trickest/trickest-cli/releases/download/v1.0.0/trickest-cli-macOS-arm64.zip

# Unzip
unzip trickest-cli-macOS-arm64.zip

# Make binary executable
chmod +x trickest-cli-macOS-arm64

# Move binary to path
mv ./trickest-cli-macOS-arm64 /usr/local/bin/trickest

# Test installation
trickest --help
```

#### **Linux**

```
curl -sLO https://github.com/trickest/trickest-cli/releases/download/v1.0.0/trickest-cli-linux-amd64.zip

# Unzip
unzip trickest-cli-linux-amd64.zip

# Make binary executable
chmod +x trickest-cli-linux-amd64

# Move binary to path
mv ./trickest-cli-linux-amd64 /usr/local/bin/trickest

# Test installation
trickest --help
```

#### Docker
```
docker run quay.io/trickest/trickest-cli
```

# Authentication

Prior to using Trickest Client, you have to enter your credentials to authenticate with the Trickest platform. For this, you will need your authentication token that can be found on [my-account page](https://trickest.io/dashboard/settings/my-account) inside the the Trickest platform.

Authentication token can be supplied as a flag `--token` or an environment variable `TRICKEST_TOKEN`.

The `TRICKEST_TOKEN` supplied as a flag will be checked **first** and take priority if both are present.

# Usage


## LIST
Use **list** command to list your private content.

#### All

Use **list** command to list all of your spaces along with their descriptions.

```
Command usage:
trickest list
```

#### Spaces

Use **list** command with **--space** option to list the content of your particular space - projects and workflows, along with their descriptions.

```
Command usage:
trickest list --space <space_name>

Flags:
--space string    The name of the space to be listed.

```

#### Projects   

Use **list** command with **--project** option to list the content of your particular project - workflows, along with their descriptions.

```
Command usage:
trickest list --project <project_name> --space <space_name>

Flags:
--project string    The name of the project to be listed.
--space string      The name of the space to which project belongs.

```

Keep in mind that when passing values that have spaces, they need be inside of double quotes (eg. "Alpine Testing")


## GET
Use **get** command to get details of your particular workflow (current status, node structure etc).

```
Command usage:
trickest get --workflow <workflow_name> --space <space_name> [--export] [--watch]

Flags:
--workflow string   The name of the workflow.
--space string      The name of the space to which workflow belongs.
--export            Option to download a workflow structure in YAML file format.
--watch             Option to track execution status in case workflow is in running state.
```


## Execute
Use **execute** command to execute a particular workflow or tool.

#### Provide parameters using **config.yaml** file

Use config.yaml file provided using **--config** option to specify:
- inputs values,
- execution parallelism by machine type,
- outputs to be downloaded.

```
Command usage:
trickest execute --workflow <workflow_or_tool_name> --space <space_name> --config <config_file_path> [--watch]

Flags:
      --config string       YAML file for run configuration
      --file string         Workflow YAML file to execute
      --max                 Use maximum number of machines for workflow execution
      --name string         New workflow name (used when creating tool workflows or copying store workflows)
      --output string       A comma separated list of nodes which outputs should be downloaded when the execution is finished
      --output-all          Download all outputs when the execution is finished
      --output-dir string   Path to directory which should be used to store outputs
      --show-params         Show parameters in the workflow tree
      --watch               Watch the execution running
      --ci                  Run in CI mode (in-progreess executions will be stopped when the CLI is forcefully stopped - if not set, you will be asked for confirmation)
```

Predefined config.yaml file content:
```
inputs:   # List of input values for the particular workflow nodes.
  <node_name>:
    <input_name>:
    - <input_value>
machines: # Machines configuration by type related to execution parallelisam.
  small:  <number>
  medium: <number>
  large:  <number>
outputs:  # List of nodes whose outputs will be downloaded.
  - <node_name>
```

Example workflow **config.yaml** files can be found in the [Trickest Workflows repository](https://github.com/trickest/workflows).

#### Provide parameters using **workflow.yaml** file

Use workflow.yaml file provided using **--file** option to specify:
- inputs values,
- execution parallelism by machine type,
- outputs to be downloaded.

Use **get** Trickest Client command along with **--export** option to download workflow.yaml file for your particular workflow. Change parameters directly in local if needed and start new execution.

For each Trickest workflow **workflow.yaml** file can be also founded in [workflows repository](https://github.com/trickest/workflows) as an example.

### Continuous Integration
You can use the `execute` command as part of a CI pipeline to continuously execute your Trickest workflows whenever your code changes. The `--watch` command can be used to watch a workflow's progress until it completes and the `--output`, `--output-all` and `--output-dir` commands can be used to fetch the outputs of one or more nodes.

The Trickest CLI Docker image is available on quay.io/trickest/trickest-cli.

Example GitHub action usage
```
  execute_trickest_workflow:
    runs-on: ubuntu-latest
    container: quay.io/trickest/trickest-cli
    env:
      TRICKEST_TOKEN: ${{ secrets.TRICKEST_TOKEN }}
    steps:
      - name: Execute workflow
        run: |
          trickest execute --workflow run_tests --space continuous_integration --config config.yaml --watch --output-all --output-dir reports
```

## Output
Use **output** command to download the outputs of your particular workflow execution(s) to local.

```
Command usage:
trickest output --workflow <workflow_name> --space <space_name> [--config <config_file_path>] [--runs <number>]

Flags:
  --workflow string            The name of the workflow.
  --space string               The name of the space to which workflow belongs.
  --config                     The file path to a config.yaml file which contains specific nodes outputs to be downloaded, otherwise all nodes will be processed.
  --runs                       The number of executions to be processed with last execution as a starting point, otherwise only last execution will be processed.
```

##### Structure

File/Directory structure will be kept the same as on the platform. Spaces and projects will become directories inside of which all of the workflow outputs will be downloaded.


## Store
[Trickest Store](https://trickest.io/dashboard/store) is a collection of all public tools, Trickest scripts and Trickest workflows available on the platform. If you are interested in viewing and executing the Trickest workflows, more info about the same you can found in Trickest [workflows repository](https://github.com/trickest/workflows).

Use **store** command to get more info about Trickest workflows and public tools available in the [Trickest Store](https://trickest.io/dashboard/store).

#### List tools
Use **store tools** command to list all public tools available in the [store](https://trickest.io/dashboard/store), along with their descriptions.

```
Command usage:
trickest store tools
```

#### List workflows
Use **store workflows** command to list all Trickest workflows available in the [store](https://trickest.io/dashboard/store), along with their descriptions.

```
Command usage:
trickest store workflows
```

#### Get tool details
Use **store get** along with **--tool** option to get the details of the specified public tool available in the [store](https://trickest.io/dashboard/store).

```
Command usage:
trickest store get --tool <tool_name>

Flags:
  --tool string         The name of the tool.
```

#### Get workflow details
Use **store get** along with **--workflow** option to get the details of the specified Trickest workflow available in the [store](https://trickest.io/dashboard/store).

```
Command usage:
trickest store get --workflow <workflow_name>

Flags:
  --workflow string     The name of the workflow.
```

#### Copy workflow to a private space
Use **store copy** command to copy particular Trickest workflow from the [store](https://trickest.io/dashboard/store) to your private space.
After copy of particular Trickest workflow is created within your private space, you can execute it using **execute** Trickest Client command.

```
Command usage:
trickest store copy --workflow <workflow_name> [--name <workflow_copy_name>] --space <space_name>

Flags:
  --workflow string         The name of the workflow to be duplicated.
  --name string             Option to set new name for workflow copy.
  --space string            The name of the space to copy workflow into. In case space doesn't exist, new space with given name will be created.

```
