"""Tests for the Python hello-world holon."""

import hello_pb2
from server import HelloServicer


def test_greet_with_name():
    servicer = HelloServicer()
    req = hello_pb2.GreetRequest(name="Bob")
    resp = servicer.Greet(req, None)
    assert resp.message == "Hello, Bob!"


def test_greet_default():
    servicer = HelloServicer()
    req = hello_pb2.GreetRequest(name="")
    resp = servicer.Greet(req, None)
    assert resp.message == "Hello, World!"
