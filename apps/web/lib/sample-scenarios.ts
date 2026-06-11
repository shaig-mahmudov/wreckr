export const checkoutRaceScenario = {
  version: 1,
  name: "checkout-idempotency-race",
  description: "Two simultaneous checkout requests with the same idempotency key must create only one order.",
  target: {
    base_url: "http://localhost:9090"
  },
  traffic: {
    type: "race",
    concurrency: 2,
    iterations: 1
  },
  setup: [
    {
      name: "reset-demo-state",
      method: "POST",
      path: "/reset",
      expect: {
        status: [200]
      }
    }
  ],
  requests: [
    {
      name: "checkout",
      method: "POST",
      path: "/checkout",
      headers: {
        "Idempotency-Key": "same-key-123"
      },
      json: {
        userId: "user-123",
        sku: "item-abc",
        quantity: 1
      },
      expect: {
        status: [201, 409]
      }
    }
  ],
  thresholds: {
    max_error_rate: 0,
    p95_ms: 750
  },
  invariants: [
    {
      name: "only-one-order-created",
      type: "http_probe",
      method: "GET",
      path: "/orders?userId=user-123&sku=item-abc",
      expect: {
        json_path: "$.count",
        equals: 1
      }
    }
  ]
};

export const loginBurstScenario = {
  version: 1,
  name: "login-burst-rate-limit",
  description: "A burst against login should degrade with 429 responses, not 500s.",
  target: {
    base_url: "http://localhost:9090"
  },
  traffic: {
    type: "burst",
    concurrency: 20,
    iterations: 40
  },
  setup: [
    {
      name: "reset-demo-state",
      method: "POST",
      path: "/reset",
      expect: {
        status: [200]
      }
    }
  ],
  requests: [
    {
      name: "login",
      method: "POST",
      path: "/login",
      json: {
        email: "user@example.com",
        password: "not-real"
      },
      expect: {
        status: [200, 429]
      }
    }
  ],
  thresholds: {
    max_error_rate: 0,
    p95_ms: 1000
  },
  invariants: [
    {
      name: "no-500s-during-rate-limit",
      type: "response_count",
      request: "login",
      status: 500,
      equals: 0
    }
  ]
};
