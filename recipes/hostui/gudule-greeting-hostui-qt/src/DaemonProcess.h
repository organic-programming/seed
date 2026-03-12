#pragma once

#include <QObject>
#include <QString>
#include <QTemporaryDir>

#include <memory>

namespace grpc {
class Channel;
}

class DaemonProcess final : public QObject {
  Q_OBJECT

public:
  explicit DaemonProcess(const QString &binaryName, QObject *parent = nullptr);

  bool start();
  void stop();

  [[nodiscard]] std::shared_ptr<grpc::Channel> channel() const;
  [[nodiscard]] QString target() const;
  [[nodiscard]] QString grpcTarget() const;
  [[nodiscard]] QString binaryPath() const;
  [[nodiscard]] QString lastError() const;

private:
  [[nodiscard]] QString resolveBinaryPath() const;
  [[nodiscard]] QString buildManifest(const QString &binaryPath) const;

  QString binaryName_;
  QString binaryPath_;
  QString grpcTarget_;
  QString lastError_;
  std::shared_ptr<grpc::Channel> channel_;
  std::unique_ptr<QTemporaryDir> stageRoot_;
};
