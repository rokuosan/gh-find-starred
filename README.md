# gh-find-starred

A tool to search for repositories you have starred on GitHub.

You can search for repositories by name and description, and collect information from **README**s.

## Prerequisites

- [gh(GitHub CLI)](https://cli.github.com/)

## Installation

```bash
$ gh extension install https://github.com/rokuosan/gh-find-starred
```

## Usage

```bash
$ gh find-starred [flags] [words...]
```

## Examples

```bash
$ gh find-starred github
```

## Flags

- `-h`, `--help`: Display help.

## Planned Flags (Not yet implemented)

- `-l`, `--limit`: Limit the number of search results displayed. The default is 10.
- `-o`, `--order`: Specify the order of search results. The default is `desc`.
- `-s`, `--sort`: Specify the sort field for search results. The default is `starred_at`.
- `-t`, `--type`: Specify the type of repositories in the search results. The default is `all`.
- `-u`, `--user`: Specify the user to search for. The default is the logged-in user.
- `-v`, `--verbose`: Display detailed information.
