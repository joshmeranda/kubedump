# KubeDump
KubeDump aims to provide k8s monitoring capabilities to the user.

## Checklist
These are the things that we need to do:
- [ ] internal dump
  - [ ] job to aggregate resource statuses / logs
  - [ ] service to expose control of job and access files created by job
  - [ ] client to query internal service
- [ ] external dump
  - [ ] repeatedly poll for resource statuses / logs