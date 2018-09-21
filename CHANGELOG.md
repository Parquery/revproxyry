# Changlog

## 1.0.5
* Root ("/") prefix is handled by revproxyry only if not specified in the configuration.

## 1.0.4
* Added the path to the config in the config error message

## 1.0.3
* Moved to github.com

## 1.0.2
* Authentication registry maps authentications by user name instead of authentication ID.

## 1.0.1
* Refactored revproxyhashry to a [separate repository](https://bitbucket.org/parqueryopen/revproxyhashry). 

  Since this change did not affect the interface of the _revproxyry_, we decided not to bump the major
  version.

* Refactored authentication, config and sigterm to a separate package
* Added component test 


## 1.0.0
* Initial version
