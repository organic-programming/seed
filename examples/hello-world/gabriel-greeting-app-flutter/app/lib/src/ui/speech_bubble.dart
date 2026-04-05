import 'package:flutter/widgets.dart';

class LeftPointerBubbleClipper extends CustomClipper<Path> {
  const LeftPointerBubbleClipper({
    this.cornerRadius = 16,
    this.pointerSize = 14,
  });

  final double cornerRadius;
  final double pointerSize;

  @override
  Path getClip(Size size) {
    final path = Path();
    final minX = pointerSize;
    final maxX = size.width;
    final minY = 0.0;
    final maxY = size.height;
    final pointerCenterY = size.height / 2;

    path.moveTo(minX + cornerRadius, minY);
    path.lineTo(maxX - cornerRadius, minY);
    path.arcToPoint(
      Offset(maxX, minY + cornerRadius),
      radius: Radius.circular(cornerRadius),
    );
    path.lineTo(maxX, maxY - cornerRadius);
    path.arcToPoint(
      Offset(maxX - cornerRadius, maxY),
      radius: Radius.circular(cornerRadius),
    );
    path.lineTo(minX + cornerRadius, maxY);
    path.arcToPoint(
      Offset(minX, maxY - cornerRadius),
      radius: Radius.circular(cornerRadius),
    );
    path.lineTo(minX, pointerCenterY + pointerSize);
    path.lineTo(0, pointerCenterY);
    path.lineTo(minX, pointerCenterY - pointerSize);
    path.lineTo(minX, minY + cornerRadius);
    path.arcToPoint(
      Offset(minX + cornerRadius, minY),
      radius: Radius.circular(cornerRadius),
    );
    path.close();
    return path;
  }

  @override
  bool shouldReclip(covariant LeftPointerBubbleClipper oldClipper) {
    return oldClipper.cornerRadius != cornerRadius ||
        oldClipper.pointerSize != pointerSize;
  }
}
