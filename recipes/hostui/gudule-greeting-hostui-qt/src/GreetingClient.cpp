#include "GreetingClient.h"

#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QJsonParseError>
#include <QProcess>

void GreetingClient::configure(const QString &binaryPath, const QString &target) {
  binaryPath_ = binaryPath;
  target_ = target;
}

QVector<GreetingLanguage> GreetingClient::listLanguages() {
  const QByteArray output = runCommand(
      {QStringLiteral("list-languages"), QStringLiteral("--target"), target_});
  if (output.isEmpty()) {
    return {};
  }

  QJsonParseError parseError;
  const QJsonDocument document =
      QJsonDocument::fromJson(output, &parseError);
  if (parseError.error != QJsonParseError::NoError || !document.isArray()) {
    lastError_ = QStringLiteral("Invalid language payload: %1")
                     .arg(parseError.errorString());
    return {};
  }

  QVector<GreetingLanguage> languages;
  const auto items = document.array();
  languages.reserve(items.size());
  for (const QJsonValue &value : items) {
    const QJsonObject object = value.toObject();
    languages.push_back(GreetingLanguage{
        object.value(QStringLiteral("code")).toString(),
        object.value(QStringLiteral("name")).toString(),
        object.value(QStringLiteral("native")).toString(),
    });
  }

  lastError_.clear();
  return languages;
}

QString GreetingClient::sayHello(const QString &name, const QString &languageCode) {
  const QByteArray output = runCommand({QStringLiteral("say-hello"),
                                        QStringLiteral("--target"), target_,
                                        QStringLiteral("--name"), name.trimmed(),
                                        QStringLiteral("--lang-code"),
                                        languageCode});
  if (output.isEmpty()) {
    return {};
  }

  QJsonParseError parseError;
  const QJsonDocument document =
      QJsonDocument::fromJson(output, &parseError);
  if (parseError.error != QJsonParseError::NoError || !document.isObject()) {
    lastError_ = QStringLiteral("Invalid greeting payload: %1")
                     .arg(parseError.errorString());
    return {};
  }

  lastError_.clear();
  return document.object().value(QStringLiteral("greeting")).toString();
}

QString GreetingClient::lastError() const { return lastError_; }

QByteArray GreetingClient::runCommand(const QStringList &arguments) {
  if (binaryPath_.isEmpty() || target_.isEmpty()) {
    lastError_ = QStringLiteral("Daemon client is not configured.");
    return {};
  }

  QProcess process;
  process.start(binaryPath_, arguments);
  if (!process.waitForFinished(5000)) {
    process.kill();
    process.waitForFinished();
    lastError_ = QStringLiteral("Daemon command timed out: %1")
                     .arg(arguments.join(QLatin1Char(' ')));
    return {};
  }

  if (process.exitStatus() != QProcess::NormalExit || process.exitCode() != 0) {
    const QString stderrText =
        QString::fromUtf8(process.readAllStandardError()).trimmed();
    lastError_ = stderrText.isEmpty()
                     ? QStringLiteral("Daemon command failed: %1")
                           .arg(arguments.join(QLatin1Char(' ')))
                     : stderrText;
    return {};
  }

  return process.readAllStandardOutput();
}
