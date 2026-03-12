#include "DaemonProcess.h"

#include <QCoreApplication>
#include <QDir>
#include <QFile>
#include <QFileInfo>

#include "holons/holons.hpp"

namespace {
constexpr auto kHolonSlug = "gudule-daemon-greeting-goqt";
constexpr auto kHolonUUID = "1a409a1e-69e3-4846-9f9b-47b0a6f98f84";
constexpr auto kFamilyName = "Greeting-Goqt";
} // namespace

DaemonProcess::DaemonProcess(const QString &binaryName, QObject *parent)
    : QObject(parent), binaryName_(binaryName) {
}

bool DaemonProcess::start() {
  if (channel_) {
    return true;
  }
  lastError_.clear();
  grpcTarget_.clear();

  const QString binaryPath = resolveBinaryPath();
  if (binaryPath.isEmpty()) {
    lastError_ = QStringLiteral("Daemon binary not found: %1").arg(binaryName_);
    return false;
  }
  binaryPath_ = binaryPath;

  auto stageRoot = std::make_unique<QTemporaryDir>(
      QDir::temp().filePath(QStringLiteral("greeting-goqt-holon-XXXXXX")));
  if (!stageRoot->isValid()) {
    lastError_ = QStringLiteral("Failed to create temporary holon root.");
    return false;
  }

  const QDir root(stageRoot->path());
  const QString holonDir =
      root.filePath(QStringLiteral("holons/%1").arg(QString::fromUtf8(kHolonSlug)));
  if (!QDir().mkpath(holonDir)) {
    lastError_ = QStringLiteral("Failed to create staged holon directory: %1").arg(holonDir);
    return false;
  }

  QFile manifest(QDir(holonDir).filePath(QStringLiteral("holon.yaml")));
  if (!manifest.open(QIODevice::WriteOnly | QIODevice::Text | QIODevice::Truncate)) {
    lastError_ = QStringLiteral("Failed to write staged holon manifest: %1")
                     .arg(manifest.errorString());
    return false;
  }
  manifest.write(buildManifest(binaryPath).toUtf8());
  manifest.close();

  const QString previousDirectory = QDir::currentPath();
  if (!QDir::setCurrent(stageRoot->path())) {
    lastError_ = QStringLiteral("Failed to enter staged holon root: %1").arg(stageRoot->path());
    return false;
  }

  try {
    channel_ = holons::connect(kHolonSlug);
    grpcTarget_ = QString::fromStdString(holons::channel_target(channel_));
    if (grpcTarget_.isEmpty()) {
      throw std::runtime_error("cpp-holons did not expose the daemon target");
    }
    stageRoot_ = std::move(stageRoot);
  } catch (const std::exception &ex) {
    lastError_ = QString::fromUtf8(ex.what());
    if (channel_) {
      try {
        holons::disconnect(channel_);
      } catch (const std::exception &) {
      }
      channel_.reset();
    }
    grpcTarget_.clear();
  }

  QDir::setCurrent(previousDirectory);
  return static_cast<bool>(channel_);
}

void DaemonProcess::stop() {
  if (channel_) {
    try {
      holons::disconnect(channel_);
    } catch (const std::exception &ex) {
      lastError_ = QString::fromUtf8(ex.what());
    }
    channel_.reset();
  }

  stageRoot_.reset();
  grpcTarget_.clear();
}

std::shared_ptr<grpc::Channel> DaemonProcess::channel() const { return channel_; }

QString DaemonProcess::target() const { return QString::fromUtf8(kHolonSlug); }

QString DaemonProcess::grpcTarget() const { return grpcTarget_; }

QString DaemonProcess::binaryPath() const { return binaryPath_; }

QString DaemonProcess::lastError() const { return lastError_; }

QString DaemonProcess::resolveBinaryPath() const {
  const QDir appDir(QCoreApplication::applicationDirPath());
  const QString bundled = appDir.filePath(QStringLiteral("daemon/%1").arg(binaryName_));
  if (QFileInfo::exists(bundled)) {
    return bundled;
  }

  const QString devPath =
      appDir.filePath(QStringLiteral("../../../../greeting-daemon/%1").arg(binaryName_));
  if (QFileInfo::exists(devPath)) {
    return QDir::cleanPath(devPath);
  }

  return {};
}

QString DaemonProcess::buildManifest(const QString &binaryPath) const {
  QString escapedPath = binaryPath;
  escapedPath.replace(QStringLiteral("\\"), QStringLiteral("\\\\"));
  escapedPath.replace(QStringLiteral("\""), QStringLiteral("\\\""));

  return QStringLiteral(
             "schema: holon/v0\n"
             "uuid: \"%1\"\n"
             "given_name: gudule-daemon\n"
             "family_name: \"%2\"\n"
             "motto: Greets users in 56 languages — a Go + Qt recipe example.\n"
             "composer: Codex\n"
             "clade: deterministic/pure\n"
             "status: draft\n"
             "born: \"2026-03-06\"\n"
             "generated_by: manual\n"
             "kind: native\n"
             "build:\n"
             "  runner: go-module\n"
             "artifacts:\n"
             "  binary: \"%3\"\n")
      .arg(QString::fromUtf8(kHolonUUID), QString::fromUtf8(kFamilyName), escapedPath);
}
