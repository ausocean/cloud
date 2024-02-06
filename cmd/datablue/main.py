import logging
from flask import Flask, redirect, request

logging.basicConfig(level=logging.INFO)

app = Flask(__name__)

targetHost = "http://vidgrind.ausocean.org/"

@app.route('/')
def home():
  logging.info("Redirecting to %s" % targetHost)
  return redirect(targetHost, code=302)

@app.route('/<path:path>', methods=['GET', 'POST'])
def other(path):
  # Redirect to the target host, preserving the path and query string.
  query_string = request.query_string.decode('utf-8')
  loc = targetHost + path + "?" + query_string
  logging.info("Redirecting to %s" % loc)
  return redirect(loc, code=302)

if __name__ == '__main__':
  app.run(host='127.0.0.1', port=8080, debug=True)
