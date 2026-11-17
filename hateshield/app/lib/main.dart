import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;

void main() => runApp(const HateShieldApp());

const apiBase = String.fromEnvironment('API_BASE', defaultValue: 'http://localhost:8080');
const apiToken = String.fromEnvironment('API_TOKEN', defaultValue: '');

Map<String, String> get _headers => {
      'Content-Type': 'application/json',
      if (apiToken.isNotEmpty) 'Authorization': 'Bearer $apiToken',
    };

class HateShieldApp extends StatelessWidget {
  const HateShieldApp({super.key});
  @override
  Widget build(BuildContext context) => MaterialApp(
        title: 'HateShield',
        theme: ThemeData(colorSchemeSeed: Colors.deepPurple, useMaterial3: true),
        home: const HomePage(),
      );
}

class HomePage extends StatefulWidget {
  const HomePage({super.key});
  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  int _tab = 0;

  @override
  Widget build(BuildContext context) => Scaffold(
        appBar: AppBar(title: const Text('HateShield')),
        body: _tab == 0 ? const AnalyzePage() : const QueuePage(),
        bottomNavigationBar: NavigationBar(
          selectedIndex: _tab,
          onDestinationSelected: (i) => setState(() => _tab = i),
          destinations: const [
            NavigationDestination(icon: Icon(Icons.search), label: 'Analyze'),
            NavigationDestination(icon: Icon(Icons.flag), label: 'Review queue'),
          ],
        ),
      );
}

class AnalyzePage extends StatefulWidget {
  const AnalyzePage({super.key});
  @override
  State<AnalyzePage> createState() => _AnalyzePageState();
}

class _AnalyzePageState extends State<AnalyzePage> {
  final _controller = TextEditingController();
  final _mediaController = TextEditingController();
  String _contentType = 'text';
  String? _result;
  bool _busy = false;

  Future<void> _analyze() async {
    setState(() { _busy = true; _result = null; });
    final body = <String, String>{'content_type': _contentType};
    if (_contentType == 'text') {
      body['text'] = _controller.text;
    } else {
      body['media_url'] = _mediaController.text;
    }
    try {
      final resp = await http.post(
        Uri.parse('$apiBase/v1/analyze'),
        headers: _headers,
        body: jsonEncode(body),
      );
      setState(() => _result = resp.body);
    } catch (e) {
      setState(() => _result = 'Error: $e');
    } finally {
      setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) => Padding(
        padding: const EdgeInsets.all(16),
        child: Column(children: [
          SegmentedButton<String>(
            segments: const [
              ButtonSegment(value: 'text', label: Text('Text')),
              ButtonSegment(value: 'audio', label: Text('Audio')),
              ButtonSegment(value: 'video', label: Text('Video')),
            ],
            selected: {_contentType},
            onSelectionChanged: (s) => setState(() => _contentType = s.first),
          ),
          const SizedBox(height: 12),
          if (_contentType == 'text')
            TextField(
              controller: _controller,
              maxLines: 4,
              decoration: const InputDecoration(
                border: OutlineInputBorder(),
                hintText: 'Paste a social media post…',
              ),
            )
          else
            TextField(
              controller: _mediaController,
              decoration: InputDecoration(
                border: const OutlineInputBorder(),
                hintText: 'https://…/media.${_contentType == 'audio' ? 'mp3' : 'mp4'}',
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
      );
}

class QueuePage extends StatefulWidget {
  const QueuePage({super.key});
  @override
  State<QueuePage> createState() => _QueuePageState();
}

class _QueuePageState extends State<QueuePage> {
  List<dynamic> _items = [];
  bool _busy = false;
  String? _error;

  Future<void> _refresh() async {
    setState(() { _busy = true; _error = null; });
    try {
      final resp = await http.get(Uri.parse('$apiBase/v1/queue'), headers: _headers);
      setState(() => _items = jsonDecode(resp.body) as List<dynamic>);
    } catch (e) {
      setState(() => _error = '$e');
    } finally {
      setState(() => _busy = false);
    }
  }

  bool _submitting = false;

  Future<void> _verdict(String postId, String label) async {
    if (_submitting) return; // prevent duplicate/conflicting labels per double-click
    setState(() => _submitting = true);
    try {
      final resp = await http.post(
        Uri.parse('$apiBase/v1/feedback'),
        headers: _headers,
        body: jsonEncode({'post_id': postId, 'label': label}),
      );
      if (resp.statusCode >= 400 && mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Verdict failed: HTTP ${resp.statusCode}')),
        );
      }
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Verdict failed: $e')),
        );
      }
    } finally {
      if (mounted) setState(() => _submitting = false);
    }
    await _refresh();
  }

  @override
  void initState() {
    super.initState();
    _refresh();
  }

  @override
  Widget build(BuildContext context) {
    if (_error != null) {
      return Center(child: Text('Error: $_error'));
    }
    return RefreshIndicator(
      onRefresh: _refresh,
      child: _items.isEmpty
          ? ListView(
              physics: const AlwaysScrollableScrollPhysics(),
              children: [
                Padding(
                  padding: const EdgeInsets.all(32),
                  child: Center(child: Text(_busy ? 'Loading…' : 'No pending items')),
                ),
              ],
            )
          : ListView.builder(
              physics: const AlwaysScrollableScrollPhysics(),
              itemCount: _items.length,
              itemBuilder: (context, i) {
                final it = _items[i] as Map<String, dynamic>;
                final scores = (it['scores'] as Map?)?.entries
                        .map((e) => '${e.key}: ${(e.value as num).toStringAsFixed(2)}')
                        .join(', ') ??
                    '';
                return Card(
                  margin: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                  child: ListTile(
                    title: Text(it['post_id']?.toString() ?? '(no id)'),
                    subtitle: Text(scores),
                    trailing: Row(mainAxisSize: MainAxisSize.min, children: [
                      IconButton(
                        icon: const Icon(Icons.check_circle, color: Colors.red),
                        tooltip: 'Confirm hate speech',
                        onPressed: () => _verdict(it['post_id']?.toString() ?? '', 'hate'),
                      ),
                      IconButton(
                        icon: const Icon(Icons.cancel, color: Colors.green),
                        tooltip: 'Not hate speech',
                        onPressed: () => _verdict(it['post_id']?.toString() ?? '', 'not_hate'),
                      ),
                    ]),
                  ),
                );
              },
            ),
    );
  }
}
