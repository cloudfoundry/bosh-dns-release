# BOSH DNS Science (Journal)

## Problem statement

The BOSH DNS Windows acceptance tests are very slow.

## Intention

The goal of this document is to methodically investigate the slow acceptance
tests. It began with this story: #165801868

## What we know

1. This used to be faster
1. We added tests
1. Our new tests also change the cloud-config
1. In the past, we've seen slowness related to changing cloud config back and forth (or not)
1. Windows bosh-dns runs test-acceptance differently from linux
1. Under windows, we run acceptance tests as a bosh errand on a windows bosh-dns VM targeting an external bosh director
1. Under linux, we spin up local (against an internal bosh director in the same concourse container) containers for running the same tests
1. 3 1/2 hours is a long time to run tests
1. some of our acceptance tests could be done as integration tests instead
1. a lot of the test time is spent spinning up VMs
1. the acceptance tests reuse the same deployment manifests for multiple tests which makes parallelization tricky right now (we think)
1. We don't have integration tests for bosh-dns
1. bbl-up takes 30 minutes since it compiles everything
1. we bbl-up every run since we need a customized bosh-director
1. we haven't changed the bosh-director code much recently
1. we have a helper script that can reuse a bosh director for windows tests
1. there seems to be a legitimate failure in the windows tests
1. the director may have changed too in the interim
1. the new tests slow down both themselves and the following test to the order of 10 minutes or so for each
1. The existing tests are slower than they used to be
1. The new tests are slower than they could be
1. within a given acceptance test context the cloud-configs are more similar
1. acceptance tests use a lot of global variables and modify state in the suite setup in a way that makes it tricky to break apart
1. it is not trivial to split out test suites

## Questions

1. Is it faster to spin up a container than a VM?
1. Why is windows failing?
1. Why don't we have integration tests for bosh-dns?
1. How difficult would it be to make these tests run in parallel?
1. Why do we try to reuse the same bosh-dns deployment for all acceptance tests?
1. What tests are slow?
1. Why do we reset the cloud-config for every test?
1. Do we need a different test setup for each test?
1. Could we reuse an environment instead of bbl-up?
1. does ginkgo randomize across test suites?
1. why is it so much faster to update with network changes on linux than on windows? Is it completely explained by the fact we are create VMs in the windows case?
1. ~~Is it weird that it takes 30 minutes to bbl-up?~~
1. ~~What tests are slow?~~
1. ~~could we minimize changes between deployments by separating the test suites trivially?~~

## Pick one

##  Theory

## Procedure

## Log
Q:
1. How can we add a simple integration test for bosh-dns?
T: 
With time and effort anything is possible.
P:
Write a test suite that bootstraps the proceses.  Notes:
  Processes needed for integration
  - bosh-dns
  - config.json for bosh-dns
  - resolv.conf
  - test-recursor-1
  - config.json for test recursor-1
  - test-recursor-2
  - config.json for test recursor-2
  For each process we need a port to listen on

Q:
1. ~~could we minimize changes between deployments by separating the test suites trivially?~~

T:
Splitting up the test suites will prevent them from running in random with each other and thereby minimize VM recreations (saving time).

P:
1. Split up the test suite
1. Run test-acceptance-windows -a to keep the environment around
1. See if these test suites are run in random with each other
