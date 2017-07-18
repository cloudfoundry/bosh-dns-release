## Background
We want to have hostnames that encode a query, like "show me a healthy database in AZ 1."

We started with the thought of encoding a query in json and base32- or base64-encoding that into the hostname. 
Example query:

```
{
  azs:[us-east-1,us-east-2],
  status:healthy,
  job:database
}
```

Removing whitespace, braces, and quotes, that query was 53 characters.
Limtation: the hostname should fit in 63 characters, so the query should fit in 61 characters.
 
## Approaches we considered:
### base32 encoding:
#### Pro:

* case insensitive 

#### Con:
* 38 character limit. `azs:[us-east-1,us-east-2],status:healthy,job:database` is 53.


### base64 encoding:
#### Pro:

* holds 45 characters, more than base32

#### Con:
* Case sensitive, where third-party software might not be
* Only holds 45 characters
* Some special characters, such as `+` or `_`


### Generate a guid for a combination of criteria
#### Pro:

* We'd be to handle a *lot* of different combinations, around 10^94.

#### Con:
* There would be state to synchronize each time a new query is generated


### gzip
#### Con:
* It added a 20-byte header, and it didn't help


### Split packet into multiple sections
e.g. `q-part1.q2-part2.rest_of_name.bosh`
#### Con:
* That wouldn't work with wildcard ssl certs; those can handle exactly one new section, not two or three


## New encoding
We'll use a short, hardcoded dictionary; the keys are one letter, and the values are numbers, drawn from not-yet-made indexes from the database. What we have so far: 

* `a` for az
* `l` for link
* `n` for network
* `s` (or `h`?) for status - 0 is healthy and the default, 1 is unhealthy, 2 is all of the above.
* `z` for Not AZ. (it's az backwards.)

The director will always specify at least the health/status query (the default query is just `q-s0`, and that is tacked onto all queries).

Sample queries:

* `a100a101n102s0` - az 100 or az 101, network 102, healthy
* `n102s0z103` - network 102, healthy, not az 103

This uses the space *much* better. We can fit a dozen items into the space we have. 
