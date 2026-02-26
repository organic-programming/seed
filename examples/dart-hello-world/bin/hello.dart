/// Pure deterministic HelloService.
String greet(String name) {
  final n = name.isEmpty ? 'World' : name;
  return 'Hello, $n!';
}

void main(List<String> args) {
  final name = args.isNotEmpty ? args.first : '';
  print(greet(name));
}
