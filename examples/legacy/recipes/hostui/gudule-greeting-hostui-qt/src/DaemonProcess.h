#pragma once

#include <QObject>
#include <QProcess>
#include <QString>
#include <QTemporaryDir>

#include <optional>
#include <memory>

namespace grpc {
class Channel;
}

class DaemonProcess final : public QObject {
  Q_OBJECT

public:
  struct GreetingDaemonIdentity {
    QString slug;
    QString familyName;
    QString binaryName;
    QString buildRunner;
    QString binaryPath;
  };

  explicit DaemonProcess(QObject *parent = nullptr);

  bool start();
  void stop();

  [[nodiscard]] std::shared_ptr<grpc::Channel> channel() const;
  [[nodiscard]] QString target() const;
  [[nodiscard]] QString grpcTarget() const;
  [[nodiscard]] QString binaryPath() const;
  [[nodiscard]] QString lastError() const;

private:
  [[nodiscard]] std::optional<GreetingDaemonIdentity> resolveDaemon() const;
  [[nodiscard]] QString buildManifest(const GreetingDaemonIdentity &daemon) const;
  [[nodiscard]] QString startBundledDaemon(const GreetingDaemonIdentity &daemon,
                                           const QString &stageRootPath);

  QString daemonSlug_;
  QString binaryPath_;
  QString grpcTarget_;
  QString lastError_;
  std::shared_ptr<grpc::Channel> channel_;
  std::unique_ptr<QTemporaryDir> stageRoot_;
  std::unique_ptr<QProcess> daemonProcess_;
};
