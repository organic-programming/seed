#include "DaemonProcess.h"

#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QFileInfo>
#include <QRegularExpression>
#include <QTextStream>

#include <set>
#include <stdexcept>

#include "holons/holons.hpp"

namespace {
constexpr auto kBinaryPrefix = "gudule-daemon-greeting-";
constexpr auto kHolonUUID = "77faf86e-e778-4cbc-a484-c545a0e35ce3";

QString displayVariant(const QString &variant) {
  const QStringList tokens = variant.split('-', Qt::SkipEmptyParts);
  QStringList parts;
  parts.reserve(tokens.size());
  for (const QString &token : tokens) {
    if (token == QStringLiteral("cpp")) {
      parts << QStringLiteral("CPP");
    } else if (token == QStringLiteral("js")) {
      parts << QStringLiteral("JS");
    } else if (token == QStringLiteral("qt")) {
      parts << QStringLiteral("Qt");
    } else if (!token.isEmpty()) {
      QString title = token;
      title[0] = title.at(0).toUpper();
      parts << title;
    }
  }
  return parts.join('-');
}

QString buildRunnerFor(const QString &variant) {
  if (variant == QStringLiteral("go")) {
    return QStringLiteral("go-module");
  }
  if (variant == QStringLiteral("rust")) {
    return QStringLiteral("cargo");
  }
  if (variant == QStringLiteral("swift")) {
    return QStringLiteral("swift-package");
  }
  if (variant == QStringLiteral("kotlin")) {
    return QStringLiteral("gradle");
  }
  if (variant == QStringLiteral("dart")) {
    return QStringLiteral("dart");
  }
  if (variant == QStringLiteral("python")) {
    return QStringLiteral("python");
  }
  if (variant == QStringLiteral("csharp")) {
    return QStringLiteral("dotnet");
  }
  if (variant == QStringLiteral("node")) {
    return QStringLiteral("npm");
  }
  return QStringLiteral("go-module");
}

std::optional<DaemonProcess::GreetingDaemonIdentity> identityFromBinaryPath(
    const QString &binaryPath) {
  const QString fileName = QFileInfo(binaryPath).fileName();
  QString normalized = fileName;
  if (normalized.endsWith(QStringLiteral(".exe"))) {
    normalized.chop(4);
  }
  if (!normalized.startsWith(QString::fromUtf8(kBinaryPrefix))) {
    return std::nullopt;
  }

  const QString variant = normalized.mid(qstrlen(kBinaryPrefix));
  return DaemonProcess::GreetingDaemonIdentity{
      QStringLiteral("gudule-greeting-daemon-%1").arg(variant),
      QStringLiteral("Greeting-Daemon-%1").arg(displayVariant(variant)),
      normalized,
      buildRunnerFor(variant),
      QFileInfo(binaryPath).absoluteFilePath(),
  };
}

void addBundledBinaries(const QString &directoryPath, QStringList *results,
                        std::set<QString> *seen) {
  const QDir directory(directoryPath);
  if (!directory.exists()) {
    return;
  }

  const QFileInfoList entries =
      directory.entryInfoList(QStringList(QStringLiteral("%1*").arg(QString::fromUtf8(kBinaryPrefix))),
                              QDir::Files | QDir::NoDotAndDotDot, QDir::Name);
  for (const QFileInfo &entry : entries) {
    const QString normalized = entry.absoluteFilePath();
    if (seen->insert(normalized).second) {
      results->append(normalized);
    }
  }
}

void addSourceTreeDaemons(const QString &directoryPath, QStringList *results,
                          std::set<QString> *seen) {
  const QDir daemonsDir(directoryPath);
  if (!daemonsDir.exists()) {
    return;
  }

  const QFileInfoList entries = daemonsDir.entryInfoList(
      QStringList(QStringLiteral("%1*").arg(QString::fromUtf8(kBinaryPrefix))),
      QDir::Dirs | QDir::NoDotAndDotDot, QDir::Name);
  for (const QFileInfo &entry : entries) {
    const QString binaryName = entry.fileName();
    const QString built =
        QDir(entry.absoluteFilePath()).filePath(QStringLiteral(".op/build/bin/%1").arg(binaryName));
    const QString local = QDir(entry.absoluteFilePath()).filePath(binaryName);
    if (seen->insert(built).second) {
      results->append(QDir::cleanPath(built));
    }
    if (seen->insert(local).second) {
      results->append(QDir::cleanPath(local));
    }
  }
}

QString firstUri(const QString &line) {
  static const QRegularExpression re(QStringLiteral(R"(tcp://\S+)"));
  const QRegularExpressionMatch match = re.match(line);
  if (!match.hasMatch()) {
    return {};
  }
  return match.captured(0);
}

QString assemblyFamily() {
  return qEnvironmentVariable("OP_ASSEMBLY_FAMILY", "Greeting-Qt-Go");
}

QString assemblyTransport() {
  return qEnvironmentVariable("OP_ASSEMBLY_TRANSPORT", "tcp");
}

QString displayConnectionTarget(const QString &value) {
  QString trimmed = value.trimmed();
  for (const auto &prefix :
       {QStringLiteral("tcp://"), QStringLiteral("http://"),
        QStringLiteral("https://"), QStringLiteral("ws://"),
        QStringLiteral("wss://"), QStringLiteral("stdio://")}) {
    if (trimmed.startsWith(prefix, Qt::CaseInsensitive)) {
      return trimmed.mid(prefix.size());
    }
  }
  return trimmed;
}

void logHostUI(const QString &line) {
  QTextStream stream(stderr);
  stream << line << Qt::endl;
}
}  // namespace

DaemonProcess::DaemonProcess(QObject *parent) : QObject(parent) {
}

bool DaemonProcess::start() {
  if (channel_) {
    return true;
  }
  lastError_.clear();
  grpcTarget_.clear();

  const auto daemon = resolveDaemon();
  if (!daemon.has_value()) {
    lastError_ = QStringLiteral("Daemon binary not found: %1*")
                     .arg(QString::fromUtf8(kBinaryPrefix));
    return false;
  }
  binaryPath_ = daemon->binaryPath;
  daemonSlug_ = daemon->slug;
  logHostUI(QStringLiteral("[HostUI] assembly=%1 daemon=%2 transport=%3")
                .arg(assemblyFamily(), daemon->binaryName, assemblyTransport()));

  auto stageRoot = std::make_unique<QTemporaryDir>(
      QDir::temp().filePath(QStringLiteral("gudule-greeting-hostui-qt-XXXXXX")));
  if (!stageRoot->isValid()) {
    lastError_ = QStringLiteral("Failed to create temporary holon root.");
    return false;
  }

  const QDir root(stageRoot->path());
  const QString holonDir =
      root.filePath(QStringLiteral("holons/%1").arg(daemon->slug));
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
  manifest.write(buildManifest(*daemon).toUtf8());
  manifest.close();

  QString portFilePath;
  try {
    portFilePath = startBundledDaemon(*daemon, stageRoot->path());
  } catch (const std::exception &ex) {
    lastError_ = QString::fromUtf8(ex.what());
    if (daemonProcess_) {
      daemonProcess_->kill();
      daemonProcess_->waitForFinished(1000);
      daemonProcess_.reset();
    }
    return false;
  }

  const QString previousDirectory = QDir::currentPath();
  if (!QDir::setCurrent(stageRoot->path())) {
    lastError_ = QStringLiteral("Failed to enter staged holon root: %1").arg(stageRoot->path());
    daemonProcess_->kill();
    daemonProcess_->waitForFinished(1000);
    daemonProcess_.reset();
    return false;
  }

  try {
    holons::ConnectOptions opts;
    opts.transport = assemblyTransport().toStdString();
    opts.start = false;
    opts.port_file = portFilePath.toStdString();
    channel_ = holons::connect(daemon->slug.toStdString(), opts);
    grpcTarget_ = QString::fromStdString(holons::channel_target(channel_));
    if (grpcTarget_.isEmpty()) {
      throw std::runtime_error("cpp-holons did not expose the daemon target");
    }
    logHostUI(QStringLiteral("[HostUI] connected to %1 on %2")
                  .arg(daemon->binaryName, displayConnectionTarget(grpcTarget_)));
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
    if (daemonProcess_) {
      daemonProcess_->kill();
      daemonProcess_->waitForFinished(1000);
      daemonProcess_.reset();
    }
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

  if (daemonProcess_) {
    daemonProcess_->kill();
    daemonProcess_->waitForFinished(1000);
    daemonProcess_.reset();
  }

  stageRoot_.reset();
  daemonSlug_.clear();
  binaryPath_.clear();
  grpcTarget_.clear();
}

std::shared_ptr<grpc::Channel> DaemonProcess::channel() const { return channel_; }

QString DaemonProcess::target() const { return daemonSlug_; }

QString DaemonProcess::grpcTarget() const { return grpcTarget_; }

QString DaemonProcess::binaryPath() const { return binaryPath_; }

QString DaemonProcess::lastError() const { return lastError_; }

std::optional<DaemonProcess::GreetingDaemonIdentity> DaemonProcess::resolveDaemon() const {
  QStringList candidates;
  std::set<QString> seen;
  const QDir currentDir(QDir::currentPath());
  addBundledBinaries(currentDir.filePath(QStringLiteral("build")), &candidates, &seen);
  addBundledBinaries(currentDir.filePath(QStringLiteral("../build")), &candidates, &seen);
  addSourceTreeDaemons(currentDir.filePath(QStringLiteral("../../daemons")), &candidates, &seen);

  const QDir appDir(QCoreApplication::applicationDirPath());
  addBundledBinaries(appDir.path(), &candidates, &seen);
  addBundledBinaries(appDir.filePath(QStringLiteral("daemon")), &candidates, &seen);
  addBundledBinaries(appDir.filePath(QStringLiteral("../Resources")), &candidates, &seen);
  addBundledBinaries(appDir.filePath(QStringLiteral("../Resources/daemon")), &candidates, &seen);

  for (const QString &candidate : candidates) {
    const QFileInfo info(candidate);
    if (!info.exists() || !info.isFile() || !info.isExecutable()) {
      continue;
    }
    if (const auto identity = identityFromBinaryPath(candidate)) {
      return identity;
    }
  }

  return std::nullopt;
}

QString DaemonProcess::startBundledDaemon(const GreetingDaemonIdentity &daemon,
                                          const QString &stageRootPath) {
  auto process = std::make_unique<QProcess>();
  process->setProgram(daemon.binaryPath);
  process->setArguments(
      {QStringLiteral("serve"), QStringLiteral("--listen"), QStringLiteral("tcp://127.0.0.1:0")});
  process->setProcessChannelMode(QProcess::MergedChannels);
  process->start();
  if (!process->waitForStarted()) {
    throw std::runtime_error("Failed to launch bundled daemon");
  }

  QString listenUri;
  QStringList recentLines;
  QElapsedTimer timer;
  timer.start();
  QByteArray buffered;

  while (timer.elapsed() < 5000 && listenUri.isEmpty()) {
    process->waitForReadyRead(100);
    buffered.append(process->readAllStandardOutput());
    const QList<QByteArray> lines = buffered.split('\n');
    buffered = lines.isEmpty() ? QByteArray() : lines.last();
    for (int i = 0; i + 1 < lines.size(); ++i) {
      const QString line = QString::fromUtf8(lines.at(i)).trimmed();
      if (line.isEmpty()) {
        continue;
      }
      recentLines << line;
      if (recentLines.size() > 8) {
        recentLines.removeFirst();
      }
      const QString candidate = firstUri(line);
      if (!candidate.isEmpty()) {
        listenUri = candidate;
        break;
      }
    }

    if (process->state() == QProcess::NotRunning && listenUri.isEmpty()) {
      throw std::runtime_error(
          QStringLiteral("Bundled daemon exited before advertising an address: %1")
              .arg(recentLines.join(QStringLiteral(" | ")))
              .toStdString());
    }
  }

  if (listenUri.isEmpty()) {
    process->kill();
    process->waitForFinished(1000);
    throw std::runtime_error("Bundled daemon did not advertise a tcp:// address");
  }

  const QString portFilePath =
      QDir(stageRootPath).filePath(QStringLiteral(".op/run/%1.port").arg(daemon.slug));
  QDir().mkpath(QFileInfo(portFilePath).absolutePath());
  QFile portFile(portFilePath);
  if (!portFile.open(QIODevice::WriteOnly | QIODevice::Text | QIODevice::Truncate)) {
    process->kill();
    process->waitForFinished(1000);
    throw std::runtime_error("Failed to write daemon port file");
  }
  QTextStream(&portFile) << listenUri << '\n';
  portFile.close();

  daemonProcess_ = std::move(process);
  return portFilePath;
}

QString DaemonProcess::buildManifest(const GreetingDaemonIdentity &daemon) const {
  QString escapedPath = daemon.binaryPath;
  escapedPath.replace(QStringLiteral("\\"), QStringLiteral("\\\\"));
  escapedPath.replace(QStringLiteral("\""), QStringLiteral("\\\""));

  return QStringLiteral(
             "schema: holon/v0\n"
             "uuid: \"%1\"\n"
             "given_name: gudule\n"
             "family_name: \"%2\"\n"
             "motto: Greets users in 56 languages through the bundled daemon.\n"
             "composer: Codex\n"
             "clade: deterministic/pure\n"
             "status: draft\n"
             "born: \"2026-03-12\"\n"
             "generated_by: manual\n"
             "kind: native\n"
             "build:\n"
             "  runner: %3\n"
             "artifacts:\n"
             "  binary: \"%4\"\n")
      .arg(QString::fromUtf8(kHolonUUID), daemon.familyName, daemon.buildRunner, escapedPath);
}
