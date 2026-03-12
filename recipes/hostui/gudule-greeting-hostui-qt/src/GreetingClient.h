#pragma once

#include <QString>
#include <QVector>

struct GreetingLanguage {
  QString code;
  QString name;
  QString nativeName;
};

class GreetingClient final {
public:
  GreetingClient() = default;

  void configure(const QString &binaryPath, const QString &target);

  [[nodiscard]] QVector<GreetingLanguage> listLanguages();
  [[nodiscard]] QString sayHello(const QString &name, const QString &languageCode);
  [[nodiscard]] QString lastError() const;

private:
  [[nodiscard]] QByteArray runCommand(const QStringList &arguments);

  QString binaryPath_;
  QString target_;
  mutable QString lastError_;
};
