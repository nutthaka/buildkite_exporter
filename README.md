**Forked from smithamax/buildkite_exporter**

## Run

    docker run --rm -p 9260:9260 -e BK_ORGANIZATION=srting_org -e BK_ACCESS_TOKEN=str_token buildkite_exporter

# Buildkite Exporter

A basic Prometheus exporter for Buildkite.

This was a learning exercise for me as I'm very new to golang, feedback/issues welcome.

## Getting Started

To run it

```
./buildkite_exporter [flags]
```

To view help

```
./buildkite_exporter --help
```

## Usage

Basic usage, token should have graphql permissions

```
buildkite_exporter \
  -buildkite.organization="someorg" \
  -buildkite.token="s3cr3t..."
```

## Metrics

| Metric                 | Labels         |
|:-----------------------|:---------------|
| buildkite_builds_total | pipeline,state |

## TODO

-   Support more than 100 pipelines (with config?, paging?)
-   Add metrics for jobs (maybe?)
-   Better build process, support --version.
