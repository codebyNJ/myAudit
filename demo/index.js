// Tiny demo app with intentional issues for myAudit to find.

export function greet(name) {
  // Bug: compares strings with == (and ignores empty name)
  if (name == null) return 'hello'
  return 'hello ' + name
}

export function total(prices) {
  let sum = 0
  // Bug: off-by-one — skips the last price
  for (let i = 0; i < prices.length - 1; i++) {
    sum += prices[i]
  }
  return sum
}

if (import.meta.url === `file://${process.argv[1]}`) {
  console.log(greet('world'), total([1, 2, 3]))
}
