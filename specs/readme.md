---
title: Update REDME# *required* (text)
assign: [] # _optional_ (list of text) Assign people by their login. Use "@me" to self-assign.
labels: # _optional_ (list of tuples) Add labels by name
  - name: # *required* (text) Label name
    color: # _optional_ (text) Color of the label
    desc: # _optional_ (text) Description of the label
milestone: # _optional_ (text) Add the issue to a milestone by name
projects: # _optional_ (list of text) Add the issue to projects by title
---

## Update the README

I need you to create the README.md that will be the only help that users of this `insitu` CLI would have to read — unless they want to contribute, then asking to read on in `CONTRIBUTING.md` and the RAG files is OK.

You should describe it's intende use, how to install (as. gh CLI extension), the YAML syntax in `.insitu.yml` etc and feel free to add links to relevant runs (believe `release.yml` is the most relevant one to track) and releases

It's important the the README is ALWAYS kept updated and relevant, therefore you must also add a general instruction to all future agent runs in either `copilot-instructions.md` or in `go-standards.instructions.md` that README-md should be reviewed on _every_ commit. And always updated accordingly if there are any feature amendments or even changes
