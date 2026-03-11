import 'package:flutter/material.dart';
import 'package:google_fonts/google_fonts.dart';

import '../../gen/greeting/v1/greeting.pb.dart';
import '../client/greeting_client.dart';

class GreetingScreen extends StatefulWidget {
  final GreetingClient client;

  const GreetingScreen({super.key, required this.client});

  @override
  State<GreetingScreen> createState() => _GreetingScreenState();
}

class _GreetingScreenState extends State<GreetingScreen>
    with SingleTickerProviderStateMixin {
  final _nameController = TextEditingController();
  List<Language> _languages = [];
  Language? _selectedLanguage;
  String? _greeting;
  String? _greetLanguage;
  bool _loading = true;
  late AnimationController _fadeController;
  late Animation<double> _fadeAnimation;

  @override
  void initState() {
    super.initState();
    _fadeController = AnimationController(
      duration: const Duration(milliseconds: 500),
      vsync: this,
    );
    _fadeAnimation = CurvedAnimation(
      parent: _fadeController,
      curve: Curves.easeIn,
    );
    _loadLanguages();
  }

  Future<void> _loadLanguages() async {
    try {
      final response = await widget.client.listLanguages();
      setState(() {
        _languages = response.languages;
        // Default to English.
        _selectedLanguage = _languages.firstWhere(
          (l) => l.code == 'en',
          orElse: () => _languages.first,
        );
        _loading = false;
      });
    } catch (e) {
      setState(() => _loading = false);
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('Failed to load languages: $e')),
        );
      }
    }
  }

  Future<void> _greet() async {
    if (_selectedLanguage == null) return;
    final name = _nameController.text.trim();
    if (name.isEmpty) return;

    _fadeController.reset();
    try {
      final response = await widget.client.sayHello(
        name,
        _selectedLanguage!.code,
      );
      setState(() {
        _greeting = response.greeting;
        _greetLanguage = response.language;
      });
      _fadeController.forward();
    } catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text('RPC error: $e')),
        );
      }
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    _fadeController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF0F0F1A),
      body: Center(
        child: Container(
          constraints: const BoxConstraints(maxWidth: 520),
          padding: const EdgeInsets.symmetric(horizontal: 32, vertical: 48),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              // Title
              Text(
                'Flutter Greeting',
                style: GoogleFonts.inter(
                  fontSize: 36,
                  fontWeight: FontWeight.w700,
                  color: Colors.white,
                ),
              ),
              const SizedBox(height: 8),
              Text(
                '56 languages | powered by Go + Flutter',
                style: GoogleFonts.inter(
                  fontSize: 14,
                  color: Colors.white54,
                ),
              ),
              const SizedBox(height: 40),

              // Language picker
              if (_loading)
                const CircularProgressIndicator(color: Colors.white54)
              else
                _buildLanguagePicker(),

              const SizedBox(height: 20),

              // Name input
              TextField(
                key: const ValueKey('name-input'),
                controller: _nameController,
                style: GoogleFonts.inter(color: Colors.white, fontSize: 18),
                decoration: InputDecoration(
                  hintText: 'Enter your name',
                  hintStyle: GoogleFonts.inter(color: Colors.white30),
                  filled: true,
                  fillColor: Colors.white.withValues(alpha: 0.06),
                  border: OutlineInputBorder(
                    borderRadius: BorderRadius.circular(12),
                    borderSide: BorderSide.none,
                  ),
                  contentPadding: const EdgeInsets.symmetric(
                    horizontal: 20,
                    vertical: 16,
                  ),
                ),
                onSubmitted: (_) => _greet(),
              ),
              const SizedBox(height: 20),

              // Greet button
              SizedBox(
                width: double.infinity,
                height: 52,
                child: ElevatedButton(
                  key: const ValueKey('greet-button'),
                  onPressed: _greet,
                  style: ElevatedButton.styleFrom(
                    backgroundColor: const Color(0xFF6C5CE7),
                    foregroundColor: Colors.white,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(12),
                    ),
                    textStyle: GoogleFonts.inter(
                      fontSize: 16,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  child: const Text('Greet'),
                ),
              ),
              const SizedBox(height: 32),

              // Greeting card
              if (_greeting != null)
                FadeTransition(
                  opacity: _fadeAnimation,
                  child: Container(
                    key: const ValueKey('greeting-output'),
                    width: double.infinity,
                    padding: const EdgeInsets.all(24),
                    decoration: BoxDecoration(
                      gradient: const LinearGradient(
                        colors: [Color(0xFF1A1A2E), Color(0xFF16213E)],
                        begin: Alignment.topLeft,
                        end: Alignment.bottomRight,
                      ),
                      borderRadius: BorderRadius.circular(16),
                      border: Border.all(
                        color: Colors.white.withValues(alpha: 0.08),
                      ),
                    ),
                    child: Column(
                      children: [
                        Text(
                          _greeting!,
                          textAlign: TextAlign.center,
                          style: GoogleFonts.inter(
                            fontSize: 28,
                            fontWeight: FontWeight.w600,
                            color: Colors.white,
                          ),
                        ),
                        const SizedBox(height: 12),
                        Text(
                          _greetLanguage ?? '',
                          style: GoogleFonts.inter(
                            fontSize: 14,
                            color: Colors.white38,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildLanguagePicker() {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      decoration: BoxDecoration(
        color: Colors.white.withValues(alpha: 0.06),
        borderRadius: BorderRadius.circular(12),
      ),
      child: DropdownButton<Language>(
        key: const ValueKey('language-picker'),
        value: _selectedLanguage,
        isExpanded: true,
        dropdownColor: const Color(0xFF1A1A2E),
        underline: const SizedBox(),
        style: GoogleFonts.inter(color: Colors.white, fontSize: 16),
        items: _languages.map((lang) {
          return DropdownMenuItem<Language>(
            value: lang,
            child: Text('${lang.name} | ${lang.native}'),
          );
        }).toList(),
        onChanged: (lang) => setState(() => _selectedLanguage = lang),
      ),
    );
  }
}
