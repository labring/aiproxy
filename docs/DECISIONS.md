# OpenAI Decisions API

AIProxy exposes the native `POST /v1/decisions` endpoint on OpenAI channels
(type `1`) and Sealos AIProxy channels. The public request and response follow the
[OpenAI Decisions API](https://developers.openai.com/api/docs/guides/decisions).

## Configure a model

1. Add an OpenAI channel with your upstream key and endpoint. The default endpoint
   is `https://api.openai.com/v1`.
2. Add a model with type **Decisions** (`41`, or `type_name: decisions` when using
   YAML configuration), or use an existing Chat Completions / Responses model
   configuration. Set the model name to `gpt-6-luna`, or use a public alias and
   map it to the upstream name in the channel's `model_mapping`.
3. Configure Decisions-specific input, cached-input, and cache-write prices.
   OpenAI currently does not charge output tokens on this endpoint. AIProxy uses
   your configured prices; it does not infer a price from the model name.

Model types and prices are configured per public model name. To expose chat and
Decisions for the same upstream model with different prices, configure separate
public aliases and map both to the same upstream model. The request endpoint
selects the protocol; the channel must implement Decisions, and OpenAI validates
whether the upstream model supports it. Model/channel tests use a native
predicate request for Decisions models; chat-mode tests do not establish
Decisions availability.

## Call the endpoint

```bash
curl https://your-aiproxy.example/v1/decisions \
  -H "Authorization: Bearer $AIPROXY_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gpt-6-luna",
    "input": [
      {"role": "user", "content": "The screen is broken."},
      {"role": "assistant", "content": "Would you like a repair or refund?"},
      {"role": "user", "content": "A refund, please."}
    ],
    "questions": [{
      "type": "choice",
      "name": "intent",
      "instructions": "Identify the current request using the full conversation.",
      "choices": [
        {"value": "refund", "description": "Request a refund"},
        {"value": "repair", "description": "Request a repair"}
      ]
    }]
  }'
```

The response is the OpenAI object with `model`, `answers`, and `usage`, without
an AIProxy data envelope. Channel model mappings are applied upstream and the
response model is restored to the public name, as on other relay endpoints.

## Compatibility and accounting

- `input` accepts text or ordered messages, function/custom tool calls, and tool
  outputs. Native image parts, JSON evidence, and content roles are preserved.
- `questions` supports `predicate`, `choice`, and `score`, including mixed types,
  question images, boolean choices, and per-question refusals. OpenAI currently
  accepts 1–400 questions; upstream validation and errors are preserved.
- `prompt_cache_key`, `prompt_cache_options`, `safety_identifier`, and additional
  JSON fields are forwarded. AIProxy's optional cache-follow plugin honors
  `prompt_cache_key` within the existing global/group channel scope.
- Responses preserve zero, false, null, probabilities, and additional fields.
  Native OpenAI error JSON and its HTTP status are also preserved. Non-JSON
  upstream failures use the normal OpenAI-compatible relay error envelope.
- Usage is read from `input_tokens`, `output_tokens`, `total_tokens`,
  `input_tokens_details.cached_tokens`, `input_tokens_details.cache_write_tokens`,
  and `output_tokens_details.reasoning_tokens`. Cache counts are already included
  in input totals and are not added again.
- Standard token authentication, model access checks, scoped channel routing,
  rate limits, retries, request logs, and configured pricing remain in effect.
- Decisions is a synchronous JSON endpoint. It does not create Responses API
  objects or expose `previous_response_id`, conversation storage, or SSE events.

See the [OpenAI reference](https://developers.openai.com/api/reference/resources/decisions/methods/create)
for the current input contract and the
[Decisions pricing guide](https://developers.openai.com/api/docs/guides/decisions#pricing-and-availability)
for model, cache, regional, and long-context pricing.
