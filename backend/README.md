# About Backend

Backend is a set of interfaces used to abstract the backend implementation of services. The goal is to make
service implementation backend agnostic, and very easy to convert between different implementations.
This is aiming to include common methods for net/http implementations and fiber implementations.

## Session

Session is an interface which describes and manages access to user sessions, which can either be stored client
side in user cookies, or server side.
