#include "gen/describe_generated.hpp"
#include "holons/composite.hpp"
#include "holons/describe.hpp"
#include "holons/serve.hpp"
#include "observability_cascade/v1/service.grpc.pb.h"
#include "relay/v1/relay.grpc.pb.h"

#include <algorithm>
#include <chrono>
#include <cctype>
#include <cstdlib>
#include <exception>
#include <filesystem>
#include <iomanip>
#include <iostream>
#include <map>
#include <memory>
#include <optional>
#include <sstream>
#include <string>
#include <vector>

using Clock = std::chrono::steady_clock;
using namespace std::chrono_literals;

namespace {

constexpr int kRunTicks = 3;
constexpr const char *kCppSlug = "observability-cascade-cpp-node";
constexpr const char *kGoSlug = "observability-cascade-go-node";

struct LanguageMember {
  std::string lang;
  std::string slug;
  std::string binary;
};

struct TickResult {
  bool pass{false};
  holons::composite::CheckOutcome hops;
  holons::composite::CheckOutcome log;
  holons::composite::CheckOutcome event;
};

struct Pattern {
  std::string name;
  std::vector<LanguageMember> members;
};

int64_t elapsed_us(Clock::time_point start) {
  auto value = std::chrono::duration_cast<std::chrono::microseconds>(
                   Clock::now() - start)
                   .count();
  return std::max<int64_t>(1, value);
}

std::string elapsed_text(int64_t us) {
  auto duration = std::chrono::microseconds(us);
  if (duration < 1s) {
    return std::to_string(
               std::chrono::duration_cast<std::chrono::milliseconds>(duration)
                   .count()) +
           "ms";
  }
  std::ostringstream out;
  out << std::fixed << std::setprecision(2)
      << std::chrono::duration<double>(duration).count() << "s";
  return out.str();
}

std::string compact(std::string value) {
  std::string out;
  bool ws = false;
  for (char ch : value) {
    if (std::isspace(static_cast<unsigned char>(ch))) {
      if (!ws) out.push_back(' ');
      ws = true;
    } else {
      out.push_back(ch);
      ws = false;
    }
  }
  if (out.size() > 240) out = out.substr(0, 240) + "...";
  return out.empty() ? "<empty>" : out;
}

std::string pass_text(bool pass) { return pass ? "PASS" : "FAIL"; }

std::string evidence_text(const holons::composite::CheckOutcome &out) {
  return out.pass ? "ok" : compact(out.evidence);
}

std::string tick_evidence(int tick, const TickResult &result) {
  return "tick=" + std::to_string(tick) + " log=" +
         evidence_text(result.log) + " event=" + evidence_text(result.event) +
         " hops=" + evidence_text(result.hops);
}

void ensure_cascade_observability() {
  auto &current = holons::observability::current();
  if (current.enabled(holons::observability::Family::Logs) &&
      current.enabled(holons::observability::Family::Events)) {
    return;
  }
  ::setenv("OP_OBS", "logs,events,metrics,prom", 1);
  ::setenv("OP_PROM_ADDR", "127.0.0.1:0", 1);
  holons::observability::from_env(
      holons::observability::Config{"observability-cascade-cpp"});
}

std::string member_binary(const std::string &id) {
  return holons::member(id).string();
}

std::vector<LanguageMember> own_language_members() {
  auto binary = member_binary("cpp-node");
  return {{ "cpp", kCppSlug, binary },
          { "cpp", kCppSlug, binary },
          { "cpp", kCppSlug, binary }};
}

std::vector<Pattern> multi_patterns() {
  auto cpp_binary = member_binary("cpp-node");
  auto go_binary = member_binary("go-node");
  std::map<std::string, LanguageMember> members = {
      {"cpp", {"cpp", kCppSlug, cpp_binary}},
      {"go", {"go", kGoSlug, go_binary}},
  };
  std::vector<Pattern> patterns;
  for (const auto &a : {"cpp", "go"}) {
    for (const auto &b : {"cpp", "go"}) {
      for (const auto &c : {"cpp", "go"}) {
        std::string name = std::string(a) + "-" + b + "-" + c;
        patterns.push_back({name, {members[a], members[b], members[c]}});
      }
    }
  }
  return patterns;
}

std::vector<holons::composite::ChildSpec>
child_specs(const std::vector<LanguageMember> &members) {
  std::vector<holons::composite::ChildSpec> out;
  out.reserve(members.size());
  for (const auto &member : members) {
    out.push_back({member.slug, member.binary});
  }
  return out;
}

std::vector<holons::composite::ChainHop>
hop_chain(const ::relay::v1::TickResponse &response) {
  std::vector<holons::composite::ChainHop> out;
  out.reserve(static_cast<size_t>(response.hops_size()));
  for (const auto &hop : response.hops()) {
    out.push_back({hop.slug(), hop.uid()});
  }
  return out;
}

holons::composite::CheckOutcome
check_hops(const ::relay::v1::TickResponse &response,
           const std::vector<LanguageMember> &members,
           std::map<std::string, int64_t> &previous) {
  if (response.hops_size() != static_cast<int>(members.size())) {
    return {false, "hops length " + std::to_string(response.hops_size()) +
                       " want " + std::to_string(members.size())};
  }
  for (int i = 0; i < response.hops_size(); ++i) {
    const auto &hop = response.hops(i);
    const auto &want = members[members.size() - 1 - static_cast<size_t>(i)];
    if (hop.slug() != want.slug) {
      return {false, "hop " + std::to_string(i) + " slug=" + hop.slug() +
                         " want " + want.slug};
    }
    if (hop.uid().empty()) {
      return {false, "hop " + std::to_string(i) + " uid empty"};
    }
    if (hop.received() <= previous[hop.uid()]) {
      return {false, "hop " + std::to_string(i) + " received=" +
                         std::to_string(hop.received()) + " previous=" +
                         std::to_string(previous[hop.uid()])};
    }
    previous[hop.uid()] = hop.received();
  }
  return {true, ""};
}

TickResult run_tick(holons::composite::Cascade &cascade,
                    const std::string &sender,
                    const std::string &note,
                    const std::vector<LanguageMember> &members,
                    std::map<std::string, int64_t> &previous,
                    bool live) {
  ::relay::v1::TickRequest request;
  request.set_sender(sender);
  request.set_note(note);
  ::relay::v1::TickResponse response;
  auto stub = ::relay::v1::RelayService::NewStub(cascade.top->channel);
  grpc::ClientContext context;
  context.set_deadline(std::chrono::system_clock::now() + 5s);
  auto status = stub->Tick(&context, request, &response);
  if (!status.ok()) {
    auto failed = holons::composite::CheckOutcome{false, status.error_message()};
    return {false, failed, failed, failed};
  }

  auto hops = check_hops(response, members, previous);
  if (!hops.pass || response.hops_size() == 0) {
    return {false, hops, {false, "skipped"}, {false, "skipped"}};
  }
  auto expected = hop_chain(response);
  auto leaf_uid = response.hops(0).uid();
  auto timeout = live ? 1000ms : 3000ms;
  auto poll = live ? 50ms : 100ms;
  auto log = holons::composite::CheckRelayedLog(
      holons::composite::LogCheckOptions{
          {},
          sender,
          leaf_uid,
          expected,
          timeout,
          poll,
          live,
      });
  auto event = holons::composite::CheckRelayedEvent(
      holons::composite::EventCheckOptions{
          {},
          holons::observability::EventType::InstanceReady,
          leaf_uid,
          expected,
          timeout,
          poll,
          live,
      });
  return {hops.pass && log.pass && event.pass, hops, log, event};
}

void add_phase(observability_cascade::v1::CascadeReport *report,
               const observability_cascade::v1::PhaseResult &phase) {
  *report->add_phases() = phase;
  report->set_pass(report->pass() + phase.pass());
  report->set_fail(report->fail() + phase.fail());
  report->set_ticks(report->pass() + report->fail());
}

void print_phase_summary(const observability_cascade::v1::PhaseResult &phase) {
  auto status = phase.fail() == 0 ? "PASS" : "FAIL";
  std::cout << "Phase " << phase.name() << ": " << phase.pass() << "/"
            << (phase.pass() + phase.fail()) << " " << status
            << " (elapsed=" << elapsed_text(phase.elapsed_us()) << ")\n";
}

observability_cascade::v1::CascadeReport
run_report(const std::string &name,
           const std::vector<LanguageMember> &members,
           bool live,
           bool emit) {
  ensure_cascade_observability();
  auto report_start = Clock::now();
  observability_cascade::v1::CascadeReport report;
  report.set_name(name);
  if (emit) {
    std::cout << "=== observability-cascade-cpp";
    if (name != "default") std::cout << " --" << name;
    std::cout << " ===\n\n";
  }

  for (size_t phase_index = 0;
       phase_index < holons::composite::TransportCoverageSequence.size();
       ++phase_index) {
    auto phase_start = Clock::now();
    const auto &transport = holons::composite::TransportCoverageSequence[phase_index];
    auto from = phase_index == 0
                    ? transport
                    : holons::composite::TransportCoverageSequence[phase_index - 1];
    std::ostringstream phase_name;
    phase_name << std::setw(2) << std::setfill('0') << (phase_index + 1)
               << "-" << from << "→" << transport;
    observability_cascade::v1::PhaseResult phase;
    phase.set_name(phase_name.str());
    if (emit) {
      std::cout << "Phase " << (phase_index + 1) << "/"
                << holons::composite::TransportCoverageSequence.size() << ": "
                << phase.name() << "\n";
    }

    std::optional<holons::composite::Cascade> cascade;
    try {
      cascade.emplace(holons::composite::BuildCascade(
          holons::composite::CascadeOptions{
              transport,
              child_specs(members),
              {{"OP_OBS", "logs,events,metrics,prom"},
               {"OP_PROM_ADDR", "127.0.0.1:0"}},
          }));
    } catch (const std::exception &error) {
      phase.set_fail(kRunTicks);
      for (int tick = 1; tick <= kRunTicks; ++tick) {
        phase.add_failures("tick=" + std::to_string(tick) +
                           " log=spawn event=spawn hops=" +
                           compact(error.what()));
      }
      phase.set_elapsed_us(elapsed_us(phase_start));
      add_phase(&report, phase);
      if (emit) print_phase_summary(phase);
      continue;
    }

    std::map<std::string, int64_t> previous;
    for (int tick = 1; tick <= kRunTicks; ++tick) {
      auto sender = name + "-phase-" + std::to_string(phase_index + 1) +
                    "-tick-" + std::to_string(tick);
      auto result = run_tick(*cascade, sender, transport, members, previous, live);
      if (result.pass) {
        phase.set_pass(phase.pass() + 1);
      } else {
        phase.set_fail(phase.fail() + 1);
        phase.add_failures(tick_evidence(tick, result));
      }
      if (emit) {
        std::cout << "  Tick " << tick << "/" << kRunTicks << ": "
                  << pass_text(result.pass) << "\n";
        if (!result.pass) {
          std::cerr << "    " << tick_evidence(tick, result) << "\n";
        }
      }
    }
    cascade->stop();
    phase.set_elapsed_us(elapsed_us(phase_start));
    add_phase(&report, phase);
    if (emit) print_phase_summary(phase);
  }
  report.set_elapsed_us(elapsed_us(report_start));
  if (emit) {
    std::cout << "\nSummary: " << report.ticks() << " ticks, " << report.pass()
              << " PASS, " << report.fail()
              << " FAIL (total elapsed=" << elapsed_text(report.elapsed_us())
              << ")\n";
  }
  return report;
}

observability_cascade::v1::MultiPatternReport run_multi_pattern_report(bool emit) {
  auto total_start = Clock::now();
  auto patterns = multi_patterns();
  observability_cascade::v1::MultiPatternReport out;
  if (emit) {
    std::cout << "=== observability-cascade-cpp --multi-pattern ===\n\n";
  }
  for (size_t i = 0; i < patterns.size(); ++i) {
    if (emit) {
      std::cout << "Pattern " << (i + 1) << "/" << patterns.size() << ": "
                << patterns[i].name << "\n";
    }
    auto report = run_report(patterns[i].name, patterns[i].members, true, emit);
    out.set_total_pass(out.total_pass() + report.pass());
    out.set_total_fail(out.total_fail() + report.fail());
    *out.add_patterns() = report;
    if (emit) {
      auto status = report.fail() == 0 ? "PASS" : "FAIL";
      std::cout << "Pattern " << patterns[i].name << ": " << report.pass()
                << "/" << report.ticks() << " " << status
                << " (elapsed=" << elapsed_text(report.elapsed_us()) << ")\n\n";
    }
  }
  out.set_total_elapsed_us(elapsed_us(total_start));
  if (emit) {
    std::cout << "Summary: " << out.total_pass() << " PASS / "
              << out.total_fail() << " FAIL across "
              << (out.total_pass() + out.total_fail())
              << " ticks (total elapsed="
              << elapsed_text(out.total_elapsed_us()) << ")\n";
  }
  return out;
}

class CascadeService final
    : public observability_cascade::v1::ObservabilityCascadeService::Service {
public:
  grpc::Status RunDefault(
      grpc::ServerContext *,
      const observability_cascade::v1::RunRequest *,
      observability_cascade::v1::CascadeReport *response) override {
    *response = run_report("default", own_language_members(), false, false);
    return grpc::Status::OK;
  }

  grpc::Status RunLiveStream(
      grpc::ServerContext *,
      const observability_cascade::v1::RunRequest *,
      observability_cascade::v1::CascadeReport *response) override {
    *response = run_report("live-stream", own_language_members(), true, false);
    return grpc::Status::OK;
  }

  grpc::Status RunMultiPattern(
      grpc::ServerContext *,
      const observability_cascade::v1::RunRequest *,
      observability_cascade::v1::MultiPatternReport *response) override {
    *response = run_multi_pattern_report(false);
    return grpc::Status::OK;
  }
};

void serve_composite(const std::vector<std::string> &args) {
  auto parsed = holons::serve::parse_options(args);
  holons::serve::options options;
  options.enable_reflection = parsed.reflect;
  options.auto_register_holon_meta = true;
  options.announce = true;
  options.slug = "observability-cascade-cpp";
  auto service = std::make_shared<CascadeService>();
  holons::serve::serve(
      parsed.listeners,
      [service](grpc::ServerBuilder &builder) {
        builder.RegisterService(service.get());
      },
      options,
      {service});
}

std::string canonical_command(std::string value) {
  std::string out;
  for (char ch : value) {
    if (ch == '-' || ch == '_' || std::isspace(static_cast<unsigned char>(ch))) {
      continue;
    }
    out.push_back(static_cast<char>(std::tolower(static_cast<unsigned char>(ch))));
  }
  return out;
}

} // namespace

int main(int argc, char **argv) {
  try {
    holons::describe::use_static_response(gen::StaticDescribeResponse());
    if (argc > 1 && canonical_command(argv[1]) == "serve") {
      std::vector<std::string> args;
      for (int i = 2; i < argc; ++i) args.emplace_back(argv[i]);
      serve_composite(args);
      return 0;
    }

    bool live = false;
    bool multi = false;
    for (int i = 1; i < argc; ++i) {
      std::string arg = argv[i];
      if (arg == "--live-stream") live = true;
      if (arg == "--multi-pattern") multi = true;
    }
    int failed = 0;
    if (multi) {
      auto report = run_multi_pattern_report(true);
      failed = report.total_fail();
    } else if (live) {
      auto report = run_report("live-stream", own_language_members(), true, true);
      failed = report.fail();
    } else {
      auto report = run_report("default", own_language_members(), false, true);
      failed = report.fail();
    }
    return failed == 0 ? 0 : 1;
  } catch (const std::exception &error) {
    std::cerr << "\nFAIL: " << error.what() << "\n";
    return 1;
  }
}
