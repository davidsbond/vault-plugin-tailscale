# vault-plugin-tailscale

[![Go Reference](https://pkg.go.dev/badge/github.com/davidsbond/vault-plugin-tailscale.svg)](https://pkg.go.dev/github.com/davidsbond/vault-plugin-tailscale)
[![Go Report Card](https://goreportcard.com/badge/github.com/davidsbond/vault-plugin-tailscale)](https://goreportcard.com/report/github.com/davidsbond/vault-plugin-tailscale)
![Github Actions](https://github.com/davidsbond/vault-plugin-tailscale/actions/workflows/ci.yml/badge.svg?branch=master)

A [HashiCorp Vault](https://www.vaultproject.io/) plugin for generating device authentication keys for 
[Tailscale](https://tailscale.com). Generated keys are single use.

## Installation

1. Download the binary for your architecture from the [releases](https://github.com/davidsbond/vault-plugin-tailscale/releases) page
2. Generate the SHA256 sum of the plugin binary

```shell
$ sha256sum vault-plugin-tailscale | cut -d ' ' -f1
d6ffe79b13326eb472af0b670c694f21f779d524068ad705a672a00f6d433724
```

3. Add the plugin to your Vault plugin catalog

```shell
$ vault plugin register -sha256=d6ffe79b13326eb472af0b670c694f21f779d524068ad705a672a00f6d433724 secret vault-plugin-tailscale
Success! Registered plugin: vault-plugin-tailscale
```

4. Enable the plugin

```shell
$ vault secrets enable -path=tailscale vault-plugin-tailscale 
Success! Enabled the vault-plugin-tailscale secrets engine at: tailscale/
```

## Usage

1. Obtain an API key from the Tailscale admin dashboard.
2. Create the Vault configuration for the Tailscale API

```shell
$ vault write tailscale/config tailnet=$TAILNET api_key=$API_KEY
Success! Data written to: tailscale/config
```

3. Generate keys using the Vault CLI.

```shell
$ vault read tailscale/key
Key          Value
---          -----
ephemeral    false
expires      2022-04-30T00:32:36Z
id           kMxzN47CNTRL
key          secret-key-data
reusable     false
tags         <nil>
```

### Key Options

The following key/value pairs can be added to the end of the `vault read` command to configure key properties:

#### Tags

Tags to apply to the device that uses the authentication key

```
vault read tailscale/key tags=something:somewhere
```

#### Preauthorized

If true, machines added to the tailnet with this key will not required authorization

```
vault read tailscale/key preauthorized=true
```

#### Ephemeral

If true, nodes created with this key will be removed after a period of inactivity or when they disconnect from the Tailnet

```
vault read tailscale/key ephemeral=true
```
