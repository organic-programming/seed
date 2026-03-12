#pragma once

#include "DaemonProcess.h"
#include "GreetingClient.h"

#include <QComboBox>
#include <QLabel>
#include <QLineEdit>
#include <QMainWindow>
#include <QPushButton>

class MainWindow final : public QMainWindow {
  Q_OBJECT

public:
  explicit MainWindow(QWidget *parent = nullptr);
  ~MainWindow() override;

private:
  void initialize();
  void refreshLanguages();
  void sayHello();

  DaemonProcess daemon_;
  GreetingClient client_;
  QLabel *statusLabel_;
  QComboBox *languageCombo_;
  QLineEdit *nameEdit_;
  QPushButton *helloButton_;
  QLabel *greetingLabel_;
};
