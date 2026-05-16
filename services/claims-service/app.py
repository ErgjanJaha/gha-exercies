from flask import Flask, jsonify

app = Flask(__name__)

CLAIMS = {
    "C-2001": {"id": "C-2001", "policy": "P-1001", "amount": 350, "status": "open"},
    "C-2002": {"id": "C-2002", "policy": "P-1002", "amount": 1200, "status": "paid"},
    "C-2003": {"id": "C-2003", "policy": "P-1003", "amount": 5400, "status": "review"},
}


@app.get("/health")
def health():
    return jsonify(status="ok", service="claims-service")


@app.get("/claims/<cid>")
def get_claim(cid):
    claim = CLAIMS.get(cid)
    if not claim:
        return jsonify(error="not found"), 404
    return jsonify(claim)


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000)
