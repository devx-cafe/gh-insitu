---
title: Support multiple runs
assign: [] # _optional_ (list of text) Assign people by their login. Use "@me" to self-assign.
labels: # _optional_ (list of tuples) Add labels by name
  - name: # *required* (text) Label name
    color: # _optional_ (text) Color of the label
    desc: # _optional_ (text) Description of the label
milestone: # _optional_ (text) Add the issue to a milestone by name
projects: # _optional_ (list of text) Add the issue to projects by title
---

## Update the `.insitu.yml` logis

I need the syntax and semantics in `insitu.yml` to be updated.

1. It must support multiple runs:

IN it's current state it has:

- an "inventory" of "checks"
- One or more waves

The is only one run - the _implied_ default - "run all waves in sequence"

But I would ike to extend the use of insitu to take over the tasks that are currently handled in

`.devcontainer/postCreateCommand` and in `.github/actions/prep-runner/action.yml`. currently they are doing basically the same thing but declared in different formats. On is designed to prep the GitHub Runners (in workflows). and the other is designed to prep the Dev Container. Two different container approaches, but containers all the same.

I would like to be able define different new "waves` that could replace
