# Configuration Apply Rules

Never edit live configuration blindly.

Required flow:

desired state
 -> render
 -> syntax validation
 -> semantic validation
 -> stage
 -> activate atomically
 -> reload
 -> health check
 -> mark version active

On failure:

restore previous known-good state
 -> reload
 -> verify health
 -> record failure

Every configuration version must have:
- version ID
- timestamp
- actor
- reason
- diff/summary
- validation result
- apply result
