.PHONY: selftest

# One-shot live self-test. Spends real Gemini tokens.
# Prerequisites: musk-back/.env LLM_* (lines 1–3), `make -C musk-back run`, then this target
# will reuse :8080 and start Vite if needed.
selftest:
	$(MAKE) -C musk-back selftest
	cd musk-web && npm run selftest
