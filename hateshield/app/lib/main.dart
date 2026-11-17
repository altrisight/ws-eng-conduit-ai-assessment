import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

void main() => runApp(const HateShieldApp());

const apiBase = String.fromEnvironment('API_BASE', defaultValue: 'http://localhost:8080');

class HateShieldApp extends StatelessWidget {
  const HateShieldApp({super.key});
  @override
  Widget build(BuildContext context) => MaterialApp(
        title: 'HateShield',
        theme: ThemeData(colorSchemeSeed: Colors.deepPurple, useMaterial3: true),
        home: const AnalyzePage(),
      );
}

class AnalyzePage extends StatefulWidget {
  const AnalyzePage({super.key});
  @override
  State<AnalyzePage> createState() => _AnalyzePageState();
}

class _AnalyzePageState extends State<AnalyzePage> {
  final _controller = TextEditingController();
  String? _result;
  bool _busy = false;

  Future<void> _analyze() async {
    setState(() { _busy = true; _result = null; });
    try {
      final resp = await http.post(
        Uri.parse('$apiBase/v1/analyze'),
        headers: {'Content-Type': 'application/json'},
        body: jsonEncode({'content_type': 'text', 'text': _controller.text}),
      );
      setState(() => _result = resp.body);
    } catch (e) {
      setState(() => _result = 'Error: $e');
    } finally {
      setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(title: const Text('HateShield')),
        body: Padding(
          padding: const EdgeInsets.all(16),
          child: Column(children: [
            TextField(
              controller: _controller,
              maxLines: 4,
              decoration: const InputDecoration(
                border: OutlineInputBorder(),
                hintText: 'Paste a social media post…',
              ),
            ),
            const SizedBox(height: 12),
            FilledButton(
              onPressed: _busy ? null : _analyze,
              child: Text(_busy ? 'Analyzing…' : 'Analyze'),
            ),
            const SizedBox(height: 12),
            if (_result != null) SelectableText(_result!),
          ]),
        ),
      );
}
