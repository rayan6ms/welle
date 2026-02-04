def abs_int(n):
  if (n < 0): return -n 
  return n

def assert_eq(test_name, a, b):
  if (a != b):
    print(f"Test {test_name} failed: {a} != {b}")
  else:
    print(f"Test {test_name} passed")
    
def is_palindrome(x):
  s = str(x)
  return s == s[::-1]

print(is_palindrome(-121))  # False
assert_eq("121", is_palindrome(121), True)
