# Examples

Verify a payment before crediting an order: start with the [payment checks in the main guide](../README.md#quick-start). Each directory here contains a runnable operation. Set `THUNDER_API_KEY_FILE` to a private file containing your service key, then read the flags or send an account-info request:

```sh
go run ./examples/get-info -h
go run ./examples/get-info -run
```

Use `-input request.json` for operation arguments, such as `payload`, `file`, `id` and `callbackUrl`. Keep the API key in the environment or a key file, never in input JSON. See each example's `-h` output for the fields it needs.

`-run` confirms that you intend to send the request. Verification can consume quota, and account examples can change your receiving accounts. Output can contain personal data; keep it private. For a receipt you plan to accept as payment, enable receiving-account, amount and duplicate checks as shown in the main guide.
